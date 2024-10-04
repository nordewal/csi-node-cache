// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/common"
)

func waitForPdCreation(ctx context.Context, t *testing.T) {
	t.Helper()
	t.Logf("%v: waiting for disks to be provisioned", time.Now())
	err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		cm, err := K8sClient.CoreV1().ConfigMaps(nodeCacheNamespace).Get(ctx, "volume-type-map", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil // retry
		} else if err != nil {
			return false, err
		}
		return strings.Contains(cm.Data["volume-types"], "disk="), nil
	})
	assert.NilError(t, err)
}

func TestPdSetup(t *testing.T) {
	skipUnlessLabeled(t, "pd")
	ctx := context.Background()
	defer testNamespaceSetup(ctx, t)()
	waitForPdCreation(ctx, t)

	pod := startCachePod(ctx, t, "mark", "pd")
	node := pod.Spec.NodeName
	if _, err := runOnPod(ctx, t, pod, "touch", "/cache/mark"); err != nil {
		t.Fatalf("Could not touch cache: %v", err)
	}
	if out, err := runOnPod(ctx, t, pod, "ls", "/cache/mark"); err != nil || !strings.Contains(out, "/cache/mark") {
		t.Fatalf("Mark didn't stick: %s / %v", out, err)
	}
	deletePod(ctx, t, pod)
	pod = startCachePodOnNode(ctx, t, "check", node)
	if out, err := runOnPod(ctx, t, pod, "ls", "/cache/mark"); err != nil || !strings.Contains(out, "/cache/mark") {
		t.Fatalf("Could not verify mark: %s / %v", out, err)
	}
}

func TestPdRepeated(t *testing.T) {
	skipUnlessLabeled(t, "pd")
	ctx := context.Background()
	defer testNamespaceSetup(ctx, t)()
	waitForPdCreation(ctx, t)

	pod := startCachePod(ctx, t, "mark", "pd")
	node := pod.Spec.NodeName
	if _, err := runOnPod(ctx, t, pod, "touch", "/cache/mark0"); err != nil {
		t.Fatalf("Could not touch cache: %v", err)
	}
	deletePod(ctx, t, pod)
	for i := 1; i < 5; i++ {
		pod = startCachePodOnNode(ctx, t, fmt.Sprintf("check-%d", i), node)
		tgt := fmt.Sprintf("/cache/mark%d", i-1)
		if out, err := runOnPod(ctx, t, pod, "ls", tgt); err != nil || !strings.Contains(out, tgt) {
			t.Fatalf("Could not verify %s: %s / %v", tgt, out, err)
		}
		if _, err := runOnPod(ctx, t, pod, "touch", fmt.Sprintf("/cache/mark%d", i)); err != nil {
			t.Fatalf("Could not touch mark%d: %v", i, err)
		}
		deletePod(ctx, t, pod)
	}
}

func TestPdMultiplePods(t *testing.T) {
	skipUnlessLabeled(t, "pd")
	ctx := context.Background()
	defer testNamespaceSetup(ctx, t)()
	waitForPdCreation(ctx, t)

	p1 := startCachePod(ctx, t, "p1", "pd")
	node := p1.Spec.NodeName
	p2 := startCachePodOnNode(ctx, t, "p2", node)
	if _, err := runOnPod(ctx, t, p1, "touch", "/cache/f1"); err != nil {
		t.Fatalf("Could not touch 1: %v", err)
	}
	if _, err := runOnPod(ctx, t, p2, "touch", "/cache/f2"); err != nil {
		t.Fatalf("Could not touch 2: %v", err)
	}
	if out, err := runOnPod(ctx, t, p1, "ls", "/cache/f2"); err != nil || !strings.Contains(out, "/cache/f2") {
		t.Fatalf("f2 didn't stick: %s", out)
	}
	if out, err := runOnPod(ctx, t, p2, "ls", "/cache/f1"); err != nil || !strings.Contains(out, "/cache/f1") {
		t.Fatalf("f2 didn't stick: %s", out)
	}
}

func TestPdPodStartedBeforeController(t *testing.T) {
	skipUnlessLabeled(t, "pd")
	ctx := context.Background()
	defer testNamespaceSetup(ctx, t)()

	mustTearDownDriver(ctx)

	t.Logf("%v: starting check pod", time.Now())
	pod := buildCmdPod("check", "/cache", map[string]string{common.VolumeTypeLabel: "pd"})
	pod, err := K8sClient.CoreV1().Pods(pod.GetNamespace()).Create(ctx, pod, metav1.CreateOptions{})
	assert.NilError(t, err)

	mustDeployDriver()

	t.Logf("%v: waiting for check pod to be running", time.Now())
	err = wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		currPod, err := K8sClient.CoreV1().Pods(pod.GetNamespace()).Get(ctx, pod.GetName(), metav1.GetOptions{})
		assert.NilError(t, err)
		assert.Assert(t, currPod.Status.Phase != corev1.PodFailed && currPod.Status.Phase != corev1.PodSucceeded)
		return currPod.Status.Phase == corev1.PodRunning, nil
	})
	assert.NilError(t, err)
}

func TestPdNodeDeletion(t *testing.T) {
	skipUnlessLabeled(t, "pd")
	ctx := context.Background()
	defer testNamespaceSetup(ctx, t)()
	waitForPdCreation(ctx, t)

	cm, err := K8sClient.CoreV1().ConfigMaps(nodeCacheNamespace).Get(ctx, "volume-type-map", metav1.GetOptions{})
	assert.NilError(t, err)
	infos := strings.Split(cm.Data["volume-types"], "\n")
	assert.Assert(t, len(infos) > 0)
	items := strings.Split(infos[0], ",")
	assert.Assert(t, len(items) > 0)
	nodeName := strings.TrimSpace(items[0])

	var pv string
	for _, i := range items {
		if !strings.Contains(i, "disk=") {
			continue
		}
		parts := strings.Split(i, "=")
		assert.Assert(t, len(parts) == 2)
		pv = strings.TrimSpace(parts[1])
		break
	}
	assert.Assert(t, pv != "")

	_, err = K8sClient.CoreV1().PersistentVolumes().Get(ctx, pv, metav1.GetOptions{})
	assert.NilError(t, err)

	deleteNode(ctx, t, nodeName)
	err = wait.PollUntilContextTimeout(ctx, time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := K8sClient.CoreV1().PersistentVolumes().Get(ctx, pv, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	assert.NilError(t, err)
}
