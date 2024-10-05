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
	"fmt"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/common"
	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/localvolume"
)

const (
	tmpfsPath  = "/local/tmpfs"
	lssdDevice = "/dev/md/lssd"
	lssdPath   = "/local/lssd"
	pdPath     = "/local/pd"

	volumeTypeInfoKey = "volume-types"
	pdVolumeType      = "pd"
)

type volumeTypeInfo struct {
	VolumeType string
	Size       resource.Quantity
	Disk       string
}

// createCacheVolume creates a volume by looking for the node in the volume type
// map and returning the appropriate local volume.
func createCacheVolume(ctx context.Context, client *kubernetes.Clientset, nodeName string, volumeTypeMapName types.NamespacedName) (localvolume.LocalVolume, error) {
	var volumeTypeMap *corev1.ConfigMap
	if err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		var err error
		volumeTypeMap, err = client.CoreV1().ConfigMaps(volumeTypeMapName.Namespace).Get(ctx, volumeTypeMapName.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to get volume type map, retrying: %v", err)
			return false, nil // retry
		}
		return true, nil
	}); err != nil {
		return nil, common.NewVolumePendingError(fmt.Errorf("no node cache volume type found: %w", err))
	}
	types, err := getVolumeTypeMapping(volumeTypeMap.Data)
	if err != nil {
		// An error means a badly formed configmap, which is terminal (not a NewVolumePendingError).
		return nil, err
	}

	info, found := types[nodeName]
	if !found {
		// An unknown type is terminal.
		return nil, common.NewVolumePendingError(fmt.Errorf("No volume type information for %s found in %s/%s", nodeName, volumeTypeMapName.Namespace, volumeTypeMapName.Name))
	}

	var vol localvolume.LocalVolume
	switch info.VolumeType {
	case "tmpfs":
		vol, err = localvolume.NewTmpfsVolume(ctx, tmpfsPath, info.Size)
	case "lssd":
		vol, err = localvolume.NewLocalSSDVolume(lssdDevice, lssdPath)
	case "pd":
		vol, err = localvolume.NewPDVolume(info.Disk, pdPath)
	default:
		err = fmt.Errorf("Unknown volume type from type info %v", info)
	}
	return vol, err
}

func getVolumeTypeMapping(configMapData map[string]string) (map[string]volumeTypeInfo, error) {
	nodes, found := configMapData[volumeTypeInfoKey]
	if !found {
		return nil, fmt.Errorf("%s not found in volume type config map", volumeTypeInfoKey)
	}
	typeMap := map[string]volumeTypeInfo{}
	for _, line := range strings.Split(nodes, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		items := strings.Split(line, ",")
		if len(items) < 2 {
			return nil, fmt.Errorf("Bad line in volume type config map: %s", line)
		}
		node := strings.TrimSpace(items[0])
		if _, found := typeMap[node]; found {
			return nil, fmt.Errorf("node %s duplicated in volume type config map: %s", node, line)
		}
		var info volumeTypeInfo
		for _, item := range items[1:] {
			parts := strings.SplitN(item, "=", 2)
			trimmed := strings.TrimSpace(parts[0])
			switch trimmed {
			case "type":
				info.VolumeType = strings.TrimSpace(parts[1])
			case "size":
				szStr := strings.TrimSpace(parts[1])
				q, err := resource.ParseQuantity(szStr)
				if err != nil {
					return nil, fmt.Errorf("bad size in volume type config map: %s", line)
				}
				info.Size = q
			case "disk":
				info.Disk = strings.TrimSpace(parts[1])
			default:
				return nil, fmt.Errorf("bad key %s in volume type config map: %s", trimmed, line)
			}
		}
		typeMap[node] = info
	}
	return typeMap, nil
}

func writeVolumeTypeMapping(configMapData map[string]string, typeMap map[string]volumeTypeInfo) error {
	lines := make([]string, 0, len(typeMap))
	for node, info := range typeMap {
		line := fmt.Sprintf("%s,type=%s", node, info.VolumeType)
		if !info.Size.IsZero() {
			line += fmt.Sprintf(",size=%s", info.Size.String())
		}
		if info.Disk != "" {
			line += fmt.Sprintf(",disk=%s", info.Disk)
		}
		lines = append(lines, line)
	}
	slices.Sort(lines)
	configMapData[volumeTypeInfoKey] = strings.Join(lines, "\n")
	return nil
}

func getVolumeTypeFromNode(node *corev1.Node) (volumeTypeInfo, error) {
	labels := node.GetLabels()
	volumeType, found := labels[common.VolumeTypeLabel]
	if !found {
		return volumeTypeInfo{}, fmt.Errorf("%s label not found on node %s", common.VolumeTypeLabel, node.GetName())
	}
	vti := volumeTypeInfo{VolumeType: volumeType}
	szStr, found := labels[common.SizeLabel]
	if found {
		q, err := resource.ParseQuantity(szStr)
		if err != nil {
			return volumeTypeInfo{}, fmt.Errorf("bad size label %s=%s on %s", common.SizeLabel, szStr, node.GetName())
		}
		vti.Size = q
	}
	return vti, nil
}
