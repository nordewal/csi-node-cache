/*
    Copyright 2023 Google LLC

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

        https://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
*/

package ramdisk

import (
	"fmt"
	"os"

	"k8s.io/mount-utils"
	"k8s.io/utils/exec"
)

// RamVolume represents a volume to the CSI node driver. It should have a
// mounter that is suitable for manipulating the volume as well as a path
// that locates the volume in the local filesystem.
type RamVolume interface {
	Mounter() *mount.SafeFormatAndMount
	Path() string
}

type emptydirRamVolume struct {
	path    string
	mounter *mount.SafeFormatAndMount
}

// NewEmptydirRamVolume creates a new empty dir ram volume, verifying its path.
func NewRamVolume(path string) (RamVolume, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("Cannot verify empty dir volume mountpoint %s: %w", path, err)
	}

	return &emptydirRamVolume{
		path:    path,
		mounter: makeMounter(),
	}, nil
}

func makeMounter() *mount.SafeFormatAndMount {
	realMounter := mount.New("")
	realExec := exec.New()
	return &mount.SafeFormatAndMount{
		Interface: realMounter,
		Exec:      realExec,
	}
}

func (v *emptydirRamVolume) Mounter() *mount.SafeFormatAndMount {
	return v.mounter
}

func (v *emptydirRamVolume) Path() string {
	return v.path
}
