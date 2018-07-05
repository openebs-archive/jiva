#!/bin/bash

CONTROLLER_IP="172.18.0.2"
REPLICA_IP1="172.18.0.3"
REPLICA_IP2="172.18.0.4"
CLONED_CONTROLLER_IP="172.18.0.5"
CLONED_REPLICA_IP="172.18.0.6"

prepare_test_env() {
	sudo rm -rf /tmp/vol*
	sudo rm -rf /mnt/logs
	sudo mkdir -p /tmp/vol1 /tmp/vol2 /tmp/vol3
	sudo mkdir -p /mnt/store /mnt/store2

	sudo docker network create --subnet=172.18.0.0/16 stg-net
	JI=$(sudo docker images | grep openebs/jiva | awk '{print $1":"$2}')
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
		if [ i == 10 ]; then
			echo "1"
		fi
	done
	echo "0"
}

# start_controller CONTROLLER_IP
start_controller() {
	controller_id=$(sudo docker run -d --net stg-net --ip "$1" -P --expose 3260 --expose 9501 --expose 9502-9504 $JI \
		launch controller --frontend gotgt --frontendIP "$1" "$2")
	echo "$controller_id"
}

# start_replica CONTROLLER_IP REPLICA_IP folder_name
start_replica() {
	replica_id=$(sudo docker run -d -it --net stg-net --ip "$2" -P --expose 9502-9504 -v /tmp/"$3":/"$3" $JI \
		launch replica --frontendIP "$1" --listen "$2":9502 --size 2g /"$3")
	echo "$replica_id"
}

# start_cloned_replica CONTROLLER_IP  CLONED_CONTROLLER_IP CLONED_REPLICA_IP folder_name
start_cloned_replica() {
	cloned_replica_id=$(sudo docker run -d -it --net stg-net --ip "$3" -P --expose 9502-9504 -v /tmp/"$4":/"$4" $JI \
		launch replica --type clone --snapName snap1 --cloneIP "$1" --frontendIP "$2" --listen "$3":9502 --size 2g /"$4")
	echo "$cloned_replica_id"
}

# get_replica_count CONTROLLER_IP
get_replica_count() {
	replicaCount=`curl http://"$1":9501/v1/volumes | jq '.data[0].replicaCount'`
	return $replicaCount
}

test_single_replica_stop_start() {
	sudo docker stop $replica1_id
	sleep 5
	sudo docker start $replica1_id
	if [ $(verify_rw_status "RW") == "0" ]; then
		echo "Single replica stop/start test passed"
	else
		echo "Single replica stop/start test failed"
		exit 1
	fi
}

test_two_replica_stop_start() {
	sudo docker stop $replica1_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 2 replicas and one is stopped"
	else
		echo "stop/start test failed when there are 2 replicas and one is stopped"
		exit 1
	fi

	sudo docker start $replica1_id
	if [ $(verify_rw_status "RW") == 0 ]; then
		echo "stop/start test passed when there are 2 replicas and one is restarted"
	else
		echo "stop/start test failed when there are 2 replicas and one is restarted"
		exit 1
	fi

	sudo docker stop $replica1_id
	sudo docker stop $replica2_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 2 replicas and both are stopped"
	else
		echo "stop/start test failed when there are 2 replicas and both are stopped"
		exit 1
	fi

	sudo docker start $replica1_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 2 replicas and one is restarted"
	else
		echo "stop/start test failed when there are 2 replicas and one is restarted"
		exit 1
	fi

	sudo docker start $replica2_id
	if [ $(verify_rw_status "RW") == 0 ]; then
		echo "Dual replica stop/start test passed"
	else
		echo "Dual replica stop/start test failed"
		exit 1
	fi

}

test_replica_reregitration() {
	i=0
	get_replica_count $CONTROLLER_IP
	while [ $? != 2 ]; do
		i=`expr $i + 1`
		if [ $i -eq 10 ]; then
			echo "Replicas failed to attach to controller"
			exit;
		fi
		echo "Wait for both replicas to attach to controller"
		sleep 5;
		get_replica_count $CONTROLLER_IP
	done

	curl -H "Content-Type: application/json" -X POST http://172.18.0.3:9502/v1/replicas/1?action=close

	i=0
	get_replica_count $CONTROLLER_IP
	while [ $? != 2 ]; do
		i=`expr $i+1`
		if [ $i -eq 10 ]; then
			echo "Closed replica failed to attach back to controller"
			exit;
		fi
		echo "Wait for the closed replica to connect back to controller"
		sleep 5;
		get_replica_count $CONTROLLER_IP
	done
}



run_vdbench_test_on_volume() {
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 5
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		sudo mount /dev/$device_name /mnt/store
		sudo mkdir -p /mnt/store/data
		sudo chown 777 /mnt/store/data
		sudo docker run -v /mnt/store/data:/datadir1 openebs/tests-vdbench:latest
		if [ $? -eq 0 ]; then echo "VDbench Test: PASSED"
		else
			echo "VDbench Test: FAILED";exit 1
		fi
		sudo umount /mnt/store
	else
		echo "Unable to detect iSCSI device, login failed"; exit 1
	fi
	logout_of_volume
}

run_libiscsi_test_suite() {
	echo "Run the libiscsi compliance suite on Jiva Vol"
	sudo mkdir /mnt/logs
	sudo docker run -v /mnt/logs:/mnt/logs --net host openebs/tests-libiscsi /bin/bash -c "./testiscsi.sh --ctrl-svc-ip $CONTROLLER_IP"
	tp=$(grep "PASSED" $(find /mnt/logs -name SUMMARY.log) | wc -l)
	tf=$(grep "FAILED" $(find /mnt/logs -name SUMMARY.log) | wc -l)
	if [ $tp -ge 146 ] && [ $tf -le 29 ]; then
		echo "iSCSI Compliance test: PASSED"
	else
		echo "iSCSI Compliance test: FAILED"; exit 1
	fi
}

logout_of_volume() {
	sudo iscsiadm -m node -u
	sudo iscsiadm -m node -o delete
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

run_data_integrity_test(){
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 5
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		sudo mkfs.ext2 -F /dev/$device_name

		sudo mount /dev/$device_name /mnt/store

		sudo dd if=/dev/urandom of=file1 bs=4k count=10000
		hash1=$(sudo md5sum file1 | awk '{print $1}')
		sudo cp file1 /mnt/store
		hash2=$(sudo md5sum /mnt/store/file1 | awk '{print $1}')
		if [ $hash1 == $hash2 ]; then echo "DI Test: PASSED"
		else
			echo "DI Test: FAILED"; exit 1
		fi

		cd /mnt/store; sync; sleep 5; sync; sleep 5; cd ~;
		sudo blockdev --flushbufs /dev/$device_name
		sudo hdparm -F /dev/$device_name
		sudo umount /mnt/store
		logout_of_volume
		sleep 5
	else
		echo "Unable to detect iSCSI device, login failed"; exit 1
	fi
}

prepare_test_env
controller_id=$(start_controller "$CONTROLLER_IP" "store1")
replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
sleep 5
test_single_replica_stop_start

replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
sleep 5
test_two_replica_stop_start
sleep 5
test_replica_reregitration
sleep 5
run_data_integrity_test
sleep 5
run_vdbench_test_on_volume
sleep 5
run_libiscsi_test_suite

