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

cd $(git rev-parse --show-toplevel)

echo "Verifying workflows..."

mod_version=$(grep ^go go.mod | sed -E 's/^go ([0-9]\.[0-9]+).*/\1/')

workflow_dir=.github/workflows
bad_versions=$(grep -H go-version ${workflow_dir}/*.yaml | grep -v "go-version: $mod_version" || true)

if [ -n "${bad_versions}" ] ; then
  echo "bad version: ${bad_versions}"
  exit 1
fi

echo "No issue found."
