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

package localvolume

import (
	"errors"
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/common"
)

func NewPDVolume(diskName, mountPath string) (LocalVolume, error) {
	if diskName == "" {
		return nil, common.NewVolumePendingError(fmt.Errorf("empty disk name"))
	}
	// This assumes the disk has been attached to the node with the device name that's the same as the disk name.
	device := fmt.Sprintf("/dev/disk/by-id/google-%s", diskName)
	if _, err := os.Stat(device); errors.Is(err, os.ErrNotExist) {
		return nil, common.NewVolumePendingError(fmt.Errorf("Waiting for attach, %s does not yet exist", device))
	}
	return NewFromDevice(device, mountPath)
}
