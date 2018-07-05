#!/bin/bash
#SIGUSR1 increases the delay by 2 seconds at certain places in controller/replica
#SIGUSR2 decreases the delay by 2 seconds at certain places in controller/replica

CONTROLLER_IP="172.18.0.2"
REPLICA_IP1="172.18.0.3"
REPLICA_IP2="172.18.0.4"
REPLICA_IP3="172.18.0.5"
CLONED_CONTROLLER_IP="172.18.0.6"
CLONED_REPLICA_IP="172.18.0.7"

prepare_test_env() {
	rm -rf /tmp/vol*
	rm -rf /mnt/logs
	mkdir -p /tmp/vol1 /tmp/vol2 /tmp/vol3 /tmp/vol4
	mkdir -p /mnt/store /mnt/store2

	docker network create --subnet=172.18.0.0/16 stg-net
	JI=$(docker images | grep openebs/jiva | awk '{print $1":"$2}')
	echo "Run CI tests on $JI"
}

# RW=1 RO=0
# verify_rw_status "RO/RW"
verify_rw_status() {
	i=0
	rw_status=""
	while [ "$rw_status" != "$1" ]; do
		sleep 5
		ro_status=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].readOnly' | tr -d '"'`
		if [ "$ro_status" == "true" ]; then
			rw_status="RO"
		elif [ "$ro_status" == "false" ]; then
			rw_status="RW"
		fi
		i=`expr $i + 1`
		if [ "$i" == 10 ]; then
			echo "1"
		fi
	done
	echo "0"
}

# start_controller CONTROLLER_IP
start_controller() {
	controller_id=$(docker run -d --net stg-net --ip "$1" -P --expose 3260 --expose 9501 --expose 9502-9504 $JI \
		launch controller --frontend gotgt --frontendIP "$1" "$2")
	echo "$controller_id"
}

# start_replica CONTROLLER_IP REPLICA_IP folder_name
start_replica() {
	replica_id=$(docker run -d -it --net stg-net --ip "$2" -P --expose 9502-9504 -v /tmp/"$3":/"$3" $JI \
		launch replica --frontendIP "$1" --listen "$2":9502 --size 2g /"$3")
	echo "$replica_id"
}

# start_cloned_replica CONTROLLER_IP  CLONED_CONTROLLER_IP CLONED_REPLICA_IP folder_name
start_cloned_replica() {
	cloned_replica_id=$(docker run -d -it --net stg-net --ip "$3" -P --expose 9502-9504 -v /tmp/"$4":/"$4" $JI \
		launch replica --type clone --snapName snap1 --cloneIP "$1" --frontendIP "$2" --listen "$3":9502 --size 2g /"$4")
	echo "$cloned_replica_id"
}

# get_replica_count CONTROLLER_IP
get_replica_count() {
	replicaCount=`curl http://"$1":9501/v1/volumes | jq '.data[0].replicaCount'`
	echo "$replicaCount"
}

test_single_replica_stop_start() {
	docker stop $replica1_id
	sleep 5
	docker start $replica1_id
	if [ $(verify_rw_status "RW") == "0" ]; then
		echo "Single replica stop/start test passed"
	else
		echo "Single replica stop/start test failed"
		exit 1
	fi
}

test_two_replica_stop_start() {
	docker stop $replica1_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 2 replicas and one is stopped"
	else
		echo "stop/start test failed when there are 2 replicas and one is stopped"
		exit 1
	fi

	docker start $replica1_id
	if [ $(verify_rw_status "RW") == 0 ]; then
		echo "stop/start test passed when there are 2 replicas and one is restarted"
	else
		echo "stop/start test failed when there are 2 replicas and one is restarted"
		exit 1
	fi

	docker stop $replica1_id
	docker stop $replica2_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 2 replicas and both are stopped"
	else
		echo "stop/start test failed when there are 2 replicas and both are stopped"
		exit 1
	fi

	docker start $replica1_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 2 replicas and one is restarted"
	else
		echo "stop/start test failed when there are 2 replicas and one is restarted"
		exit 1
	fi

	docker start $replica2_id
	if [ $(verify_rw_status "RW") == 0 ]; then
		echo "Dual replica stop/start test passed"
	else
		echo "Dual replica stop/start test failed"
		exit 1
	fi

}
run_ios_to_test_stop_start() {
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 2
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		# Add 4 sec delay in serving IOs from replica1, start IOs, and then close replica1
		# This will trigger the quorum condition which checks if the IOs are
		# written to more than 50% of the replicas
		docker kill --signal=SIGUSR1 $replica1_id
		docker kill --signal=SIGUSR1 $replica1_id

		dd if=/dev/urandom of=/dev/$device_name bs=4k count=1000
		if [ $? -eq 0 ]; then echo "IOs were written successfully while running 3 replicas stop/start test"
		else
			echo "IOs errored out while running 3 replicas stop/start test"; exit 1
		fi
		logout_of_volume
		sleep 5
	else
		echo "Unable to detect iSCSI device, login failed"; exit 1
	fi
}

test_three_replica_stop_start() {
	run_ios_to_test_stop_start &
	sleep 8
	docker stop $replica1_id
	if [ $(verify_rw_status "RW") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and one is stopped"
	else
		echo "stop/start test failed when there are 3 replicas and one is stopped"
		exit 1
	fi
	docker stop $replica2_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and two are stopped"
	else
		echo "stop/start test failed when there are 3 replicas and two are stopped"
		exit 1
	fi

	docker stop $replica3_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and all are stopped"
	else
		echo "stop/start test failed when there are 3 replicas and all are stopped"
		exit 1
	fi

	docker start $replica1_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and one is restarted"
	else
		echo "stop/start test failed when there are 3 replicas and one is restarted"
		exit 1
	fi
	# Sending SIGUSR1 to controller twice adds delay of 4 seconds in the controller code at some places
	# It has been introduced just before starting 2nd and 3rd replica.
	# Path being tested here is the race condition which should be hit as both the replicas 
	# are added at the same time at the controller. So that both the replicas are opened but 
	# only one is attached successfully and the other is left in open state.
	docker kill --signal=SIGUSR1 $controller_id
	docker kill --signal=SIGUSR1 $controller_id

	docker start $replica2_id
	if [ $(verify_rw_status "RW") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and two are restarted"
	else
		echo "stop/start test failed when there are 3 replicas and two are restarted"
		exit 1
	fi

	docker start $replica3_id
	if [ $(verify_rw_status "RW") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and all are restarted"
	else
		echo "stop/start test failed when there are 3 replicas and all are restarted"
		exit 1
	fi

	replica_count=$(get_replica_count $CONTROLLER_IP)
	while [ "$replica_count" != 3 ]; do
		i=`expr $i + 1`
		if [ $i -eq 10 ]; then
			echo "Closed replica failed to attach back to controller"
			exit;
		fi
		echo "Wait for the closed replica to connect back to controller, replicaCount: "$replica_count
		sleep 5;
		replica_count=$(get_replica_count $CONTROLLER_IP)
	done

}

test_replica_reregitration() {
	i=0
	replica_count=$(get_replica_count $CONTROLLER_IP)
	while [ "$replica_count" != 3 ]; do
		i=`expr $i + 1`
		if [ $i -eq 10 ]; then
			echo "Replicas failed to attach to controller"
			exit;
		fi
		echo "Wait for the closed replica to connect back to controller, replicaCount: "$replica_count
		sleep 5;
		replica_count=$(get_replica_count $CONTROLLER_IP)
	done

	curl -H "Content-Type: application/json" -X POST http://172.18.0.3:9502/v1/replicas/1?action=close

	i=0
	replica_count=$(get_replica_count $CONTROLLER_IP)
	while [ "$replica_count" != 3 ]; do
		i=`expr $i + 1`
		if [ $i -eq 10 ]; then
			echo "Closed replica failed to attach back to controller"
			exit;
		fi
		echo "Wait for the closed replica to connect back to controller, replicaCount: "$replica_count
		sleep 5;
		replica_count=$(get_replica_count $CONTROLLER_IP)
	done
}



run_vdbench_test_on_volume() {
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 5
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		mount /dev/$device_name /mnt/store
		mkdir -p /mnt/store/data
		chown 777 /mnt/store/data
		docker run -v /mnt/store/data:/datadir1 openebs/tests-vdbench:latest
		if [ $? -eq 0 ]; then echo "VDbench Test: PASSED"
		else
			echo "VDbench Test: FAILED";exit 1
		fi
		umount /mnt/store
	else
		echo "Unable to detect iSCSI device, login failed"; exit 1
	fi
	logout_of_volume
}

run_libiscsi_test_suite() {
	echo "Run the libiscsi compliance suite on Jiva Vol"
	mkdir /mnt/logs
	docker run -v /mnt/logs:/mnt/logs --net host openebs/tests-libiscsi /bin/bash -c "./testiscsi.sh --ctrl-svc-ip $CONTROLLER_IP"
	tp=$(grep "PASSED" $(find /mnt/logs -name SUMMARY.log) | wc -l)
	tf=$(grep "FAILED" $(find /mnt/logs -name SUMMARY.log) | wc -l)
	if [ $tp -ge 146 ] && [ $tf -le 29 ]; then
		echo "iSCSI Compliance test: PASSED"
	else
		echo "iSCSI Compliance test: FAILED"; exit 1
	fi
}

logout_of_volume() {
	iscsiadm -m node -u
	iscsiadm -m node -o delete
}

# Discover Jiva iSCSI target and Login
login_to_volume() {
	iscsiadm -m discovery -t st -p $1
	iscsiadm -m node -l
}

# Wait for iSCSI device node (scsi device) to be created
get_scsi_disk() {
	device_name=$(iscsiadm -m session -P 3 |grep -i "Attached scsi disk" | awk '{print $4}')
	i=0
	while [ -z $device_name ]; do
		sleep 5
		device_name=$(iscsiadm -m session -P 3 |grep -i "Attached scsi disk" | awk '{print $4}')
		i=`expr $i + 1`
		if [ $i -eq 10 ]; then
			echo "scsi disk not found";
			exit;
		else
			continue;
		fi
	done
}

run_data_integrity_test() {
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 5
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		mkfs.ext2 -F /dev/$device_name

		mount /dev/$device_name /mnt/store

		dd if=/dev/urandom of=file1 bs=4k count=10000
		hash1=$(md5sum file1 | awk '{print $1}')
		cp file1 /mnt/store
		hash2=$(md5sum /mnt/store/file1 | awk '{print $1}')
		if [ $hash1 == $hash2 ]; then echo "DI Test: PASSED"
		else
			echo "DI Test: FAILED"; exit 1
		fi

		cd /mnt/store; sync; sleep 5; sync; sleep 5; cd ~;
		blockdev --flushbufs /dev/$device_name
		hdparm -F /dev/$device_name
		umount /mnt/store
		logout_of_volume
		sleep 5
	else
		echo "Unable to detect iSCSI device, login failed"; exit 1
	fi
}

create_snapshot() {
	id=`curl http://$1:9501/v1/volumes | jq '.data[0].id' |  tr -d '"'`
	curl -H "Content-Type: application/json" -X POST -d '{"name":"snap1"}' http://$CONTROLLER_IP:9501/v1/volumes/$id?action=snapshot
}

test_clone_feature() {
	start_controller "$CLONED_CONTROLLER_IP" "store2"
	start_cloned_replica "$CONTROLLER_IP"  "$CLONED_CONTROLLER_IP" "$CLONED_REPLICA_IP" "vol4"

	if [ $(verify_clone_status "completed") == "0" ]; then
		echo "clone created successfully"
	else
		echo "Clone creation failed"
		exit 1
	fi

	login_to_volume "$CLONED_CONTROLLER_IP:3260"
	get_scsi_disk

	if [ "$device_name"!="" ]; then
		mount /dev/$device_name /mnt/store2

		hash3=$(md5sum /mnt/store2/file1 | awk '{print $1}')
		if [ $hash1 == $hash3 ]; then
			umount /mnt/store2
			logout_of_volume
			echo "DI Test: PASSED"
		else
			umount /mnt/store2
			logout_of_volume
			echo "DI Test: FAILED"; exit 1
		fi
	else
		echo "Unable to detect iSCSI device, login failed"; exit 1
	fi
}

verify_clone_status() {
	i=0
	clonestatus=""
	while [ "$clonestatus" != "$1" ]; do
		sleep 5
		clonestatus=`curl http://$CLONED_REPLICA_IP:9502/v1/replicas/1 | jq '.clonestatus' | tr -d '"'`
		i=`expr $i + 1`
		if [ $i -eq 20 ]; then
			echo "1"
			return
		else
			continue
		fi
	done
	echo "0"
}

prepare_test_env
controller_id=$(start_controller "$CONTROLLER_IP" "store1")
replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
sleep 5
test_single_replica_stop_start
sleep 5
replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
sleep 5
test_two_replica_stop_start
sleep 5
replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")
sleep 5
test_three_replica_stop_start
sleep 5
test_replica_reregitration
sleep 5
run_data_integrity_test
sleep 5
run_vdbench_test_on_volume
sleep 5
run_libiscsi_test_suite

