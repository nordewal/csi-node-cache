# CSI Ram Disk

This repository contains the source code for a CSI ephemeral volume implemented by a node
local ram disk. A single volume is created per node, so all pods on the same node will see
the same volume. If a pod is deleted and another pod scheduled to the same node, it will
see the previous contents of volume (but see below). This makes this volume good for
caching of stateful data for workloads that expect replicas to be rescheduled to the same
node after upgrades or other restarts.

This volume is **not** resilient to CSI driver restarts. If the driver restarts, behavior
is undefined. The contents of the ram volume in this case may disappear at any time ---
although in practice if a pod is using the volume, it will not be destroyed until the pod
exits.

# Usage

See [`examples/rampod.yaml`](examples/rampod.yaml). The volume is defined as a CSI
ephemeral volume using the `ramdisk.csi.storage.gke.io` driver name (this can be changed
by adjusting [`deploy/csidriver.yaml`](deploy/csidriver.yaml) and the `driverName`
constant in [`pkg/ramdisk/identity.go`](pkg/ramdisk/identity.go)).

# Deployment

  1. Build an image using the [`Dockerfile`](Dockerfile) and push it to the repository of
     your choice (`docker build . -t $YOUR-IMAGE-REPO` is enough to build, the Dockerfile
     isn't fancy).
  1. Update the `csi-ramdisk` container image in [`deploy/node.yaml`](deploy/node.yaml) to
     the image you just built. If you've changed the driver name, update the
     `--kubelet-registration-path` flag in that file as well as the `CSIDriver` name in
     [`deploy/csidriver.yaml`](deploy/csidriver.yaml).
  1. `kubectl apply -f deploy/` to install the driver on your cluster.
  
The driver is deployed as a `DaemonSet`. There is no clsuter-wide controller pod, all CSI
ephemeral volume processing is done by the node-local `DaemonSet`. Note that the driver
pods on nodes, like all CSI drivers, runs as privileged.
