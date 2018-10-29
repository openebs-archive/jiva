#!/bin/bash

#imports
. ci/libs/variables.sh
. ci/libs/utils.sh

test_single_replica_stop_start() {
	echo "----------------Test_single_replica_stop_start--------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "1")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	sleep 5

	verify_replica_cnt "1" "Single replica count test"

	docker stop $replica1_id
	sleep 5

	verify_vol_status "RO" "Single replica stop test"

	docker start $replica1_id

	verify_vol_status "RW" "Single replica start test"
	verify_replica_cnt "1" "Single replica count test"
	verify_controller_rep_state "$REPLICA_IP1" "RW" "Single replica status during start test"
	docker stop $replica1_id
	docker stop $orig_controller_id
	cleanup
}

test_single_replica_stop_start
