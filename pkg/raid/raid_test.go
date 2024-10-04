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

package raid

import (
	"reflect"
	"slices"
	"testing"
)

func TestGetInactiveDevices(t *testing.T) {
	tests := []struct {
		output          string
		expectedDevices []string
	}{
		{
			output: `Personalities : [raid1]
md127 : inactive sdb[3](S)
      10484736 blocks super 1.2

unused devices: <none>
`,
			expectedDevices: []string{"/dev/md127"},
		},
		{
			output: `Personalities : [raid1]
unused devices: <none>
`,
			expectedDevices: []string{},
		},
		{
			output: `Personalities : [raid1]
md127 : inactive sdb[3](S)
      10484736 blocks super 1.2

md126 : inactive ram0[3](S)
      10484736 blocks super 1.2
`,
			expectedDevices: []string{"/dev/md127", "/dev/md126"},
		},
		{
			output: `Personalities : [raid1]
md127 : active raid1 sdd[1] ram0[0]
      130048 blocks super 1.2 [2/2] [UU]

unused devices: <none>
`,
			expectedDevices: []string{},
		},
		{
			output: `Personalities : [raid1]
md127 : active raid1 sdd[1] ram0[0]
      130048 blocks super 1.2 [2/2] [UU]

md126 : inactive ram0[3](S)
      10484736 blocks super 1.2
`,
			expectedDevices: []string{"/dev/md126"},
		},
	}
	for _, test := range tests {
		devices := getInactiveDevices(test.output)
		slices.Sort(devices)
		slices.Sort(test.expectedDevices)
		if !reflect.DeepEqual(devices, test.expectedDevices) {
			t.Errorf("Got %v expected %v for %s", devices, test.expectedDevices, test.output)
		}
	}
}
