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
test_two_replica_stop_start
test_three_replica_stop_start
test_write_io_timeout
test_write_io_timeout_with_readwrite_env
test_replica_restart_optimization
test_replica_restart_while_snap_deletion
test_single_replica_stop_start
test_restart_during_prepare_rebuild
test_ctrl_stop_start
test_replica_controller_continuous_stop_start
#test_bad_file_descriptor
test_replica_rpc_close
test_controller_rpc_close
#test_replication_factor
#test_two_replica_delete
#test_replica_ip_change
#test_replica_reregistration
test_duplicate_snapshot_failure
#test_extent_support_file_system
run_vdbench_test_on_volume
