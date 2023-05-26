# Copyright 2023 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.20.3-bullseye AS builder
WORKDIR /src
ADD . .
RUN go build ./cmd/ramdisk

FROM registry.k8s.io/build-image/debian-base:bullseye-v1.4.3 AS debian
RUN clean-install mount bash

# Since we're leveraging apt to pull in dependencies, we use 
# `gcr.io/distroless/base` because it includes glibc.
FROM gcr.io/distroless/base-debian11 as distroless

COPY --from=builder /src/ramdisk /ramdisk

COPY --from=debian /bin/mount /bin/mount
COPY --from=debian /bin/umount /bin/umount

COPY --from=debian \
    /lib/x86_64-linux-gnu/libselinux.so.1 \
    /usr/lib/x86_64-linux-gnu/libblkid.so.1 \
    /usr/lib/x86_64-linux-gnu/libpcre2-8.so.0 \
    /usr/lib/x86_64-linux-gnu/libmount.so.1 \
    /lib/x86_64-linux-gnu/

FROM distroless AS check

COPY --from=debian /usr/bin/ldd /usr/bin/find /usr/bin/xargs /usr/bin/
COPY --from=debian /bin/bash /bin/grep /bin/
COPY --from=builder /src/hack/print-missing-deps.sh /print-missing-deps.sh

COPY --from=debian \
    /lib/x86_64-linux-gnu/libpcre.so.3 \
    /lib/x86_64-linux-gnu/libtinfo.so.6 \
    /lib/x86_64-linux-gnu/

SHELL ["/bin/bash", "-c"]
RUN /print-missing-deps.sh

# Final output with entrypoint
FROM distroless
ENTRYPOINT ["/ramdisk"]
