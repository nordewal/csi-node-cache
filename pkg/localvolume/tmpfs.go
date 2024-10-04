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
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/mount-utils"
	"k8s.io/utils/exec"
)

type tmpfsVolume struct {
	path string
}

var _ LocalVolume = &tmpfsVolume{}

// NewTmpfsVolume makes a new ram volume based on a tmpfs mounted to path.  The
// tmpfs creation happens at the time of this call, and an error will be
// returned if the mount fails. The tmpfs is created with hugepages. path is
// created if it doesn't already exist.
func NewTmpfsVolume(ctx context.Context, path string, size resource.Quantity) (LocalVolume, error) {
	if size.IsZero() {
		return nil, fmt.Errorf("Bad size %v", size)
	}

	if err := os.MkdirAll(path, 0750); err != nil {
		return nil, fmt.Errorf("Could not use or create %s: %w", path, err)
	}

	mountOpts := []string{
		fmt.Sprintf("size=%dM", int64(size.AsApproximateFloat64()/1024/1024)),
		fmt.Sprintf("huge=always"),
	}

	mounter := &mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      exec.New(),
	}
	if err := mounter.Mount("tmpfs", path, "tmpfs", mountOpts); err != nil {
		return nil, fmt.Errorf("Could not mount at %s with %v: %w", path, mountOpts, err)
	}

	return &tmpfsVolume{
		path: path,
	}, nil

}

func (v *tmpfsVolume) Path() string {
	return v.path
}
