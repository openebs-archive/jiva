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
