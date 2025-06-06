#!/bin/bash
# Copyright 2024 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -o errexit
set -o nounset
set -o pipefail

PKG_ROOT=$(git rev-parse --show-toplevel)

"${PKG_ROOT}"/hack/verify-workflows.sh
"${PKG_ROOT}"/hack/verify-gofmt.sh
"${PKG_ROOT}"/hack/verify-trailing-whitespace.sh
"${PKG_ROOT}"/hack/verify-govet.sh
"${PKG_ROOT}"/hack/verify-gomod.sh
