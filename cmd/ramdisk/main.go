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

package main

import (
	"flag"

	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/csi-ramdisk/pkg/ramdisk"
)

var (
	endpoint      = flag.String("endpoint", "unix:/tmp/csi.sock", "CSI endpoint")
	ramEmptydir   = flag.String("ram-emptydir", "", "Location of ram disk. It is an error if this does not exist")
	nodeId        = flag.String("node-id", "", "Node id returned during registration process, should be the pod spec.nodeName")
)

func init() {
	// klog verbosity guide for this package
	// Use V(2) for one time config information
	// Use V(4) for general debug information logging
	// Use V(5) for GCE Cloud Provider Call informational logging
	// Use V(6) for extra repeated/polling information
	klog.InitFlags(flag.CommandLine)
	flag.Set("logtostderr", "true")
}

func main() {
	flag.Parse()

	if len(*ramEmptydir) == 0 {
		klog.Fatalf("Must specify --ram-emptydir")
	}

	var volume ramdisk.RamVolume
	var err error

	if volume, err = ramdisk.NewRamVolume(*ramEmptydir); err != nil {
		klog.Fatalf("Could not make ram volume at emptyDir %s: %v", *ramEmptydir, err)
	}
	klog.V(4).Infof("Created emptyDir ram volume at %s", *ramEmptydir)

	klog.V(4).Infof("Creating driver on %s", *nodeId)
	driver, err := ramdisk.NewDriver(*endpoint, *nodeId, volume)
	if err != nil {
		klog.Fatalf("Cannot create driver: %v", err)
	}

	if err := driver.Run(); err != nil {
		klog.Fatalf("Error while running driver: %v", err)
	}
}
