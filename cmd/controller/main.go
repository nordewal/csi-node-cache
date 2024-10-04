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
	"context"
	"flag"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/GoogleCloudPlatform/csi-node-cache/pkg/csi"
)

var (
	namespace      = flag.String("namespace", "", "Namespace for worker pods")
	volumeTypeMap  = flag.String("volume-type-map", "", "The name of the volume type config map, found in --namespace")
	pdStorageClass = flag.String("pd-storage-class", "", "The storage class to use for the PD cache type. If empty, PD caches cannot be used")

	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
	flag.Parse()

	ctx := context.Background()

	problem := false
	if *namespace == "" {
		setupLog.Error(nil, "missing --namespace")
		problem = true
	}

	if *volumeTypeMap == "" {
		setupLog.Error(nil, "missing --volume-type-map")
		problem = true
	}

	if problem {
		os.Exit(1)
	}

	csi.ControllerInit()

	cfg := ctrl.GetConfigOrDie()

	var attacher csi.Attacher
	if *pdStorageClass != "" {
		var err error
		attacher, err = csi.NewAttacher(ctx, cfg)
		if err != nil {
			setupLog.Error(err, "getting attacher")
			os.Exit(1)
		}
	}

	mgr, err := csi.NewManager(cfg, *namespace, *volumeTypeMap, attacher, *pdStorageClass)
	if err != nil {
		setupLog.Error(err, "new manager creation")
		os.Exit(1)
	}
	setupLog.Info("starting manager")
	err = mgr.Start(ctrl.SetupSignalHandler())
	setupLog.Error(err, "unexpected manager exit")
	os.Exit(1)
}
