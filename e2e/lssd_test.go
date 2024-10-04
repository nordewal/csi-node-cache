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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/common"
)

func listFilesOnNode(ctx context.Context, t *testing.T, node, glob string) []string {
	output, err := runOnNode(ctx, t, node, "bash", "-c", fmt.Sprintf("\"ls %s\"", glob))
	if err != nil {
		t.Fatalf("bad ls %s on %s: %v", glob, node, err)
	}
	files := []string{}
	for _, l := range strings.Split(output, "\n") {
		for _, i := range strings.Split(l, " ") {
			f := strings.TrimSpace(i)
			if f != "" {
				files = append(files, f)
			}
		}
	}
	return files
}

func initializeRaidNodes(ctx context.Context, t *testing.T) {
	t.Helper()

	restartDriver(ctx, t)

	nodes, err := K8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("can't list nodes for initializing raid: %v", err)
	}
	for _, node := range nodes.Items {
		if kind, found := node.GetLabels()[common.VolumeTypeLabel]; !found || kind != "lssd" {
			continue
		}
		runOnNode(ctx, t, node.GetName(), "umount", "/dev/md/lssd")
		runOnNode(ctx, t, node.GetName(), "mdadm", "--stop", "/dev/md/lssd")
		for _, f := range listFilesOnNode(ctx, t, node.GetName(), "/dev/md[0-9]*") {
			runOnNode(ctx, t, node.GetName(), "umount", f)
			runOnNode(ctx, t, node.GetName(), "mdadm", "--stop", f)
		}
		for _, f := range listFilesOnNode(ctx, t, node.GetName(), "/dev/disk/by-id/google-local-ssd-block*") {
			runOnNode(ctx, t, node.GetName(), "mdadm", "--zero-superblock", f)
		}
	}
}

func TestLssdRaidSetup(t *testing.T) {
	skipUnlessLabeled(t, "lssd")
	ctx := context.Background()
	initializeRaidNodes(ctx, t)
	defer testNamespaceSetup(ctx, t)()

	pod := startCachePod(ctx, t, "mark", "lssd")
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

func TestLssdMultiplePods(t *testing.T) {
	skipUnlessLabeled(t, "lssd")
	ctx := context.Background()
	initializeRaidNodes(ctx, t)
	defer testNamespaceSetup(ctx, t)()

	p1 := startCachePod(ctx, t, "p1", "lssd")
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
