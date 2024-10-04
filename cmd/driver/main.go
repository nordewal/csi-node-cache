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

package main

import (
	"flag"

	"k8s.io/klog/v2"

	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/csi"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	driverVersion string // Set during build

	endpoint      = flag.String("endpoint", "unix:/tmp/csi.sock", "CSI endpoint")
	nodeName      = flag.String("node-name", "", "The node name, probably pod spec.NodeName.")
	namespace     = flag.String("namespace", "", "The namespace of the driver & the volume type map.")
	volumeTypeMap = flag.String("volume-type-map", "", "The name of the volume type config map used by the controller")
	driverName    = flag.String("driver-name", "", "The driver name as specified in the CSIDriver object.")
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

	if *nodeName == "" {
		klog.Fatalf("Missing --node-name")
	}
	if *namespace == "" {
		klog.Fatalf("Missing --namespace")
	}
	if *volumeTypeMap == "" {
		klog.Fatalf("Missing --volume-type-map")
	}
	if *driverName == "" {
		klog.Fatalf("Missing --driver-name")
	}

	client, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		klog.Fatalf("could not create kubeclient: %v", err)
	}

	klog.V(4).Infof("Creating driver on %s", *nodeName)
	driver, err := csi.NewDriver(client, *endpoint, *nodeName, types.NamespacedName{Namespace: *namespace, Name: *volumeTypeMap}, *driverName, driverVersion)
	if err != nil {
		klog.Fatalf("Cannot create driver: %v", err)
	}

	err = driver.Run()
	klog.Fatalf("Driver or server unexpectedly exited, with error %v", err)
}
