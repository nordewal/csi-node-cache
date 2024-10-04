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

package csi

import (
	"context"
	"flag"
	"fmt"
	"gotest.tools/v3/assert"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/common"
)

const (
	controllerNamespace = "csi-node-cache"
	mappingConfigMap    = "volume-info"

	WaitInterval = 1 * time.Second
	WaitTimeout  = 15 * time.Second

	pdStorageClass = "a-storage-class"

	attachLabel = "fake-attached-to"
)

var (
	k8sClient client.Client
	testCfg   *rest.Config

	skipControllerTests = false
)

type fakeAttacher struct {
	k8sClient client.Client
}

func (a *fakeAttacher) diskIsAttached(ctx context.Context, volume, nodename string) (bool, error) {
	vol, err := parseVolumeHandle(volume)
	if err != nil {
		return false, err
	}
	var pv corev1.PersistentVolume
	if err := a.k8sClient.Get(ctx, types.NamespacedName{Name: vol.name}, &pv); err != nil {
		return false, err
	}
	_, found := pv.GetLabels()[attachLabel]
	return found, nil
}

func (a *fakeAttacher) attachDisk(ctx context.Context, volume, nodeName string) error {
	vol, err := parseVolumeHandle(volume)
	if err != nil {
		return err
	}
	var pv corev1.PersistentVolume
	if err := a.k8sClient.Get(ctx, types.NamespacedName{Name: vol.name}, &pv); err != nil {
		return err
	}
	labels := pv.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[attachLabel] = nodeName
	pv.SetLabels(labels)
	return a.k8sClient.Update(ctx, &pv)
}

func setupEnviron(ctx context.Context) {
	log := log.FromContext(ctx)
	kubeRoot := os.Getenv("KUBE_ROOT")
	fmt.Printf("kube root is %s\n", kubeRoot) // If I don't do this, kubeRoot is nil????
	if kubeRoot == "" {
		log.Error(fmt.Errorf("Missing KUBE_ROOT"), "KUBE_ROOT should be set, and should point to a kubernetes installation with etcd and api server built, from hack/install-etcd.sh and make quick-release. If they aren't present, testing will fail with errors about not being able to find those binaries. For now relevant tests will be skipped")
		skipControllerTests = true
		return
	}
	os.Setenv("TEST_ASSET_ETCD", filepath.Join(kubeRoot, "third_party/etcd/etcd"))
	os.Setenv("TEST_ASSET_KUBE_APISERVER", filepath.Join(kubeRoot, "_output/release-stage/server/linux-amd64/kubernetes/server/bin/kube-apiserver"))
}

func TestMain(m *testing.M) {
	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	setupEnviron(context.TODO())

	ControllerInit() // Setup the scheme

	m.Run()
}

func mustSetupCluster() (context.Context, func(ctx context.Context)) {
	ctx, globalCancel := context.WithCancel(context.TODO())
	log := log.FromContext(ctx)

	testEnv := &envtest.Environment{
		UseExistingCluster: ptr.To(false),
	}
	var err error
	testCfg, err = testEnv.Start()
	if err != nil {
		log.Error(err, "cannot start testenv")
		os.Exit(1)
	}

	k8sClient, err = client.New(testCfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Error(err, "cannot create client")
		os.Exit(1)
	}

	manager, err := NewManager(testCfg, controllerNamespace, mappingConfigMap, &fakeAttacher{k8sClient}, pdStorageClass)
	if err != nil {
		log.Error(err, "cannot setup manager")
		os.Exit(1)
	}

	go func() {
		if err := manager.Start(ctx); err != nil {
			log.Error(err, "manager startup")
			os.Exit(1)
		}
	}()

	var ns corev1.Namespace
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: controllerNamespace}, &ns); apierrors.IsNotFound(err) {
		ns.SetName(controllerNamespace)
		if err := k8sClient.Create(ctx, &ns); err != nil {
			log.Error(err, "Can't create namespace", "namespace", controllerNamespace)
			os.Exit(1)
		}
	}

	return ctx, func(_ context.Context) {
		globalCancel()
		if err := testEnv.Stop(); err != nil {
			log.Error(err, "testenv shutdown")
			os.Exit(1)
		}
	}
}

func createNode(ctx context.Context, t *testing.T, name string, labels map[string]string) *corev1.Node {
	t.Helper()
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	err := k8sClient.Create(ctx, &node)
	assert.NilError(t, err)
	return &node
}

func fetchNodeMapping(ctx context.Context, t *testing.T, node string) (volumeTypeInfo, error) {
	var foundInfo volumeTypeInfo
	err := wait.PollUntilContextTimeout(ctx, WaitInterval, WaitTimeout, true, func(ctx context.Context) (bool, error) {
		var configMap corev1.ConfigMap
		err := k8sClient.Get(ctx, types.NamespacedName{Name: mappingConfigMap, Namespace: controllerNamespace}, &configMap)
		if apierrors.IsNotFound(err) {
			return false, nil // retry
		}
		if err != nil {
			return false, err
		}
		typeInfo, err := getVolumeTypeMapping(configMap.Data)
		if err != nil {
			return false, err
		}
		info, found := typeInfo[node]
		if !found {
			return false, nil // retry
		}
		foundInfo = info
		return true, nil
	})
	return foundInfo, err
}

func waitForNodeMapping(ctx context.Context, t *testing.T, node string) volumeTypeInfo {
	t.Helper()
	info, err := fetchNodeMapping(ctx, t, node)
	assert.NilError(t, err)
	return info
}

func assertNoMapping(ctx context.Context, t *testing.T, node string) {
	t.Helper()
	_, err := fetchNodeMapping(ctx, t, node)
	assert.ErrorContains(t, err, "context deadline")
}

func TestSomeNodes(t *testing.T) {
	if skipControllerTests {
		t.Skip("Skipping controller test")
	}

	ctx, cleanup := mustSetupCluster()

	createNode(ctx, t, "a", map[string]string{common.VolumeTypeLabel: "foo"})
	createNode(ctx, t, "b", map[string]string{"someOtherlabel": "bar"})
	createNode(ctx, t, "c", map[string]string{common.VolumeTypeLabel: "baz"})

	info := waitForNodeMapping(ctx, t, "a")
	assert.Equal(t, info.VolumeType, "foo")
	info = waitForNodeMapping(ctx, t, "c")
	assert.Equal(t, info.VolumeType, "baz")
	assertNoMapping(ctx, t, "b")

	cleanup(ctx)
}

func TestPdNode(t *testing.T) {
	if skipControllerTests {
		t.Skip("Skipping controller test")
	}

	ctx, cleanup := mustSetupCluster()

	createNode(ctx, t, "a", map[string]string{common.VolumeTypeLabel: "pd", common.SizeLabel: "50Gi"})
	err := wait.PollUntilContextTimeout(ctx, WaitInterval, WaitTimeout, true, func(ctx context.Context) (bool, error) {
		var pvc corev1.PersistentVolumeClaim
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: controllerNamespace, Name: "a"}, &pvc)
		if apierrors.IsNotFound(err) {
			return false, nil // retry
		} else if err != nil {
			return false, err
		}
		if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != pdStorageClass {
			return false, fmt.Errorf("Unexpected storageclass %v", pvc.Spec.StorageClassName)
		}
		if pvc.Status.Phase != corev1.ClaimBound {
			pvName := "pv-for-" + pvc.GetName()
			pv := corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: pvName,
				},
				Spec: corev1.PersistentVolumeSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Capacity:    pvc.Spec.Resources.Requests,
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       "dont-care",
							VolumeHandle: fmt.Sprintf("project/unknown/zones/unknown/disks/%s", pvName),
						},
					},
				},
			}
			if err := k8sClient.Create(ctx, &pv); err != nil {
				return false, err
			}
			pvc.Spec.VolumeName = pv.GetName()
			if err := k8sClient.Update(ctx, &pvc); err != nil {
				return false, err
			}
			pvc.Status.Phase = corev1.ClaimBound
			if err := k8sClient.Status().Update(ctx, &pvc); err != nil {
				return false, err
			}
			return false, nil // retry to give our controller time to update from the PVC.
		}
		// Our fake attacher labels the PV.
		var pv corev1.PersistentVolume
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: "pv-for-a"}, &pv); err != nil {
			return false, err
		}
		node, found := pv.GetLabels()[attachLabel]
		if !found {
			return false, nil // retry
		}
		if node != "a" {
			return false, fmt.Errorf("Unexpectedly attached to %s instead of a", node)
		}
		return true, nil
	})

	assert.NilError(t, err, "volume not created & attached to node a")

	cleanup(ctx)
}
