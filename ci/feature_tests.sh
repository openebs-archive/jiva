# Copyright © 2020 The OpenEBS Authors
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

source ./ci/suite.sh

if [ -z "$ARCH" ]; then
  echo "platform not specified for running tests. Exiting."
  exit 1
fi

# currently integration tests are run only for amd64
if [ "$ARCH" != "linux_amd64" ]; then
  echo "skipping test for $ARCH"
  exit 0
fi

prepare_test_env
test_max_chain_env
test_volume_resize
test_delete_snapshot
test_duplicate_data_delete
test_preload
run_data_integrity_test_with_fs_creation
test_clone_feature
run_libiscsi_test_suite
test_upgrades
