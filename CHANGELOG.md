# Changelog and Version History

## v1.0.0

* Move to new localvolume organization.

* tmpfs and local ssd support only.

## v0.1.0

* sshfs / remote volume support, including service/labeling controller.

## v0.0.1

* hugepage support; explicit tmpfs rather than emptyDir. This makes
  resource allocation harder but performance can be 50% better (1-2
  G/s to 2-3 G/s).

## v0.0.0

* initial version based on emptyDir tmpfs.
