# End-to-end Testing

These tests assume a cluster reachable by kubeconfig. `make images` should be
run before starting these tests as the driver will be deployed on this cluster
with the current setup.

Nodes should be labeled with the kind of node cache as described in
[../README.md](../README.md), which will guide which tests will be run.
