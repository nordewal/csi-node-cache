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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/raid"
)

// NewLocalSSDVolume raids up all local ssd volumes and returns the formatted device.
func NewLocalSSDVolume(raidDevice, mountPath string) (LocalVolume, error) {
	devices, err := getLocalSSDs()
	if err != nil {
		return nil, err
	}
	array := raid.NewStripedArray(raidDevice, devices...)
	if err := array.Init(); err != nil {
		return nil, err
	}
	return NewFromDevice(raidDevice, mountPath)
}

func getLocalSSDs() ([]string, error) {
	// on n4, boot disk is /dev/sda
	// /dev/nvme0  /dev/nvme0n1  /dev/nvme0n2	/dev/nvme0n3  /dev/nvme0n4
	//
	// Also there is $ ls -l /dev/disk/by-id/google-local-ssd*
	// lrwxrwxrwx 1 root root 39 Jul 12 18:28 /dev/disk/by-id/google-local-ssd-block0 -> /dev/disk/by-id/google-local-nvme-ssd-0
	// lrwxrwxrwx 1 root root 39 Jul 12 18:28 /dev/disk/by-id/google-local-ssd-block1 -> /dev/disk/by-id/google-local-nvme-ssd-1
	// lrwxrwxrwx 1 root root 39 Jul 12 18:28 /dev/disk/by-id/google-local-ssd-block2 -> /dev/disk/by-id/google-local-nvme-ssd-2
	// lrwxrwxrwx 1 root root 39 Jul 12 18:28 /dev/disk/by-id/google-local-ssd-block3 -> /dev/disk/by-id/google-local-nvme-ssd-3
	//
	// on c3d-standard-16-lssd (which has one card)
	// /dev/nvme0    /dev/nvme0n1p1   /dev/nvme0n1p11	/dev/nvme0n1p2	/dev/nvme0n1p4	/dev/nvme0n1p6	/dev/nvme0n1p8	/dev/nvme1
	// /dev/nvme0n1  /dev/nvme0n1p10  /dev/nvme0n1p12	/dev/nvme0n1p3	/dev/nvme0n1p5	/dev/nvme0n1p7	/dev/nvme0n1p9	/dev/nvme1n1
	//
	// /dev/disk/by-id has
	// lrwxrwxrwx 1 root root 13 Jul 12 17:53 nvme-nvme_card-pd_nvme_card-pd -> ../../nvme0n1
	// lrwxrwxrwx 1 root root 13 Jul 12 17:53 nvme-nvme_card0_nvme_card0 -> ../../nvme1n1
	// lrwxrwxrwx 1 root root 13 Jul 12 17:53 nvme-nvme_card0_nvme_card0_1 -> ../../nvme1n1
	//
	// The nvme0 namespace is the boot disk.
	//
	// On a c3d-standard-60-lssd, which has 4 cards:
	//
	// /dev/disk/by-id/nvme-nvme_card0_nvme_card0    /dev/disk/by-id/nvme-nvme_card1_nvme_card1_1  /dev/disk/by-id/nvme-nvme_card3_nvme_card3
	// /dev/disk/by-id/nvme-nvme_card0_nvme_card0_1  /dev/disk/by-id/nvme-nvme_card2_nvme_card2    /dev/disk/by-id/nvme-nvme_card3_nvme_card3_1
	// /dev/disk/by-id/nvme-nvme_card1_nvme_card1    /dev/disk/by-id/nvme-nvme_card2_nvme_card2_1
	//
	// And the boot disk is nvme-nvme_card-pd*
	//
	// When attaching another hyperdisk, it appears at /dev/disk/by-id/google-persistent-disk-5 -> ../../nvme0n2
	//
	// and we have nvme-nvme_card-pd_nvme_card-pd_1 -> ../../nvme0n1, /dev/disk/by-id/nvme-nvme_card-pd_nvme_card-pd_2 -> ../../nvme0n2
	//
	// finially, there are
	// $ ls -l /dev/disk/by-id/google-local-*
	// lrwxrwxrwx 1 root root 13 Jul 12 17:58 /dev/disk/by-id/google-local-nvme-ssd-0 -> ../../nvme1n1
	// lrwxrwxrwx 1 root root 13 Jul 12 17:58 /dev/disk/by-id/google-local-nvme-ssd-1 -> ../../nvme2n1
	// lrwxrwxrwx 1 root root 13 Jul 12 17:58 /dev/disk/by-id/google-local-nvme-ssd-2 -> ../../nvme3n1
	// lrwxrwxrwx 1 root root 13 Jul 12 17:58 /dev/disk/by-id/google-local-nvme-ssd-3 -> ../../nvme4n1
	// lrwxrwxrwx 1 root root 39 Jul 12 17:58 /dev/disk/by-id/google-local-ssd-block0 -> /dev/disk/by-id/google-local-nvme-ssd-0
	// lrwxrwxrwx 1 root root 39 Jul 12 17:58 /dev/disk/by-id/google-local-ssd-block1 -> /dev/disk/by-id/google-local-nvme-ssd-1
	// lrwxrwxrwx 1 root root 39 Jul 12 17:58 /dev/disk/by-id/google-local-ssd-block2 -> /dev/disk/by-id/google-local-nvme-ssd-2
	// lrwxrwxrwx 1 root root 39 Jul 12 17:58 /dev/disk/by-id/google-local-ssd-block3 -> /dev/disk/by-id/google-local-nvme-ssd-3
	//
	// Whereas the attached disks are all google-persistent-disk-*.
	//
	// So we'll use /dev/disk/by-id/google-local-ssd-block*

	entries, err := fs.ReadDir(os.DirFS("/dev/disk"), "by-id")
	if err != nil {
		return nil, err
	}
	devices := []string{}
	for _, f := range entries {
		if strings.HasPrefix(f.Name(), "google-local-ssd-block") {
			devices = append(devices, filepath.Join("/dev/disk/by-id", f.Name()))
		}
	}
	return devices, nil
}
