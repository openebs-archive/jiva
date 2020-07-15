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
