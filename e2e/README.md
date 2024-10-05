# End-to-end Testing

These tests assume a cluster reachable by kubeconfig. `make images` should be
run before starting these tests as the driver will be deployed on this cluster
with the current setup.

Nodes should be labeled with the kind of node cache as described in
[../README.md](../README.md), which will guide which tests will be run. Remember
that for PD and tmpfs caches, node-cache-size.gke.io should also be set.

The PD test will delete a node, so the node label should be set on the node-pool
with gcloud rather than with kubectl. That way when the node is recreated it
will be labeled correctly.

The tests assume running on a GKE cluster, and use `gcloud` to manipulate
resources. `gcloud compute ssh` is used. Depending on your machine setup, you
may have to add something like the following to your `~/.ssh/config` file.

```
Host *
  ProxyCommand /usr/bin/corp-ssh-helper -dst_username=%r %h %p
```
