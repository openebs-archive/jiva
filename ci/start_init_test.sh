#!/bin/bash
set -x
CONTROLLER_IP="172.18.0.2"
REPLICA_IP1="172.18.0.3"
REPLICA_IP2="172.18.0.4"
REPLICA_IP3="172.18.0.5"
CLONED_CONTROLLER_IP="172.18.0.6"
CLONED_REPLICA_IP="172.18.0.7"
REPLICATION_FACTOR=0

collect_logs_and_exit() {
	echo "--------------------------docker ps -a-------------------------------------"
	docker ps -a

	#Below is to get stack traces of longhorn processes
	#kill -SIGABRT $(ps -auxwww | grep -w longhorn | grep -v grep | awk '{print $2}')

	echo "--------------------------ORIGINAL CONTROLLER LOGS ------------------------"
	curl http://$CONTROLLER_IP:9501/v1/volumes | jq
	curl http://$CONTROLLER_IP:9501/v1/replicas | jq
	docker logs $orig_controller_id
	echo "--------------------------REPLICA 1 LOGS ----------------------------------"
	curl http://$REPLICA_IP1:9502/v1/replicas | jq
	docker logs $replica1_id
	echo "--------------------------REPLICA 2 LOGS ----------------------------------"
	curl http://$REPLICA_IP2:9502/v1/replicas | jq
	docker logs $replica2_id
	echo "--------------------------REPLICA 3  LOGS ---------------------------------"
	curl http://$REPLICA_IP3:9502/v1/replicas | jq
	docker logs $replica3_id
	echo "--------------------------CLONED CONTROLLER LOGS --------------------------"
	docker logs $cloned_controller_id
	echo "--------------------------CLONED REPLICA LOGS -----------------------------"
	docker logs $cloned_replica_id
	exit 1
}

prepare_test_env() {
	rm -rf /tmp/vol*
	rm -rf /mnt/logs
	docker stop $(docker ps -aq)
	docker rm $(docker ps -aq)

	mkdir -p /tmp/vol1 /tmp/vol2 /tmp/vol3 /tmp/vol4
	mkdir -p /mnt/store /mnt/store2

	docker network create --subnet=172.18.0.0/16 stg-net
	JI=$(docker images | grep openebs/jiva | awk '{print $1":"$2}' | head -1)
	echo "Run CI tests on $JI"
}

verify_replica_cnt() {
	i=0
	replica_cnt=""
	while [ "$replica_cnt" != "$1" ]; do
		date
		replica_cnt=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].replicaCount'`
		i=`expr $i + 1`
		if [ "$i" == 10 ]; then
			echo $2 " failed"
			collect_logs_and_exit
		fi
		sleep 4
	done
	echo $2 " passed"
	return
}

# RW=1 RO=0
# verify_rw_status "RO/RW"
verify_rw_status() {
	i=0
	rw_status=""
	while [ "$rw_status" != "$1" ]; do
		date
		ro_status=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].readOnly' | tr -d '"'`
		if [ "$ro_status" == "true" ]; then
			rw_status="RO"
		elif [ "$ro_status" == "false" ]; then
			rw_status="RW"
		fi
		i=`expr $i + 1`
		if [ "$i" == 10 ]; then
			echo "1"
			return
		fi
		sleep 4
	done
	echo "0"
}

verify_vol_status() {
	i=0
	rw_status=""
	while [ "$rw_status" != "$1" ]; do
		date
		ro_status=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].readOnly' | tr -d '"'`
		if [ "$ro_status" == "true" ]; then
			rw_status="RO"
		elif [ "$ro_status" == "false" ]; then
			rw_status="RW"
		fi
		i=`expr $i + 1`
		if [ "$i" == 10 ]; then
			echo $2 " failed"
			collect_logs_and_exit
		fi
		sleep 4
	done
	echo $2 " passed"
	return
}

verify_rep_state() {
	i=0
	rep_state=""
	while [ "$i" != 10 ]; do
		date
		rep_cnt=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].replicaCount'`
		replica_cnt=`expr $rep_cnt`
		passed=0
		#if [ "$replica_cnt" == 0 ]; then
			rep_state=`curl http://$3:9502/v1/replicas | jq '.data[0].state' | tr -d '"'`
			if [ "$rep_state" == "closed" ]; then
				passed=`expr $passed + 1`
			fi
			if [ "$5" != "" ]; then
				rep_state=`curl http://$5:9502/v1/replicas | jq '.data[0].state' | tr -d '"'`
				if [ "$rep_state" == "closed" ]; then
					passed=`expr $passed + 1`
				fi
			fi
		#fi
		rep_index=0
		while [ $rep_index -lt $replica_cnt ]; do
			address=`curl http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].address' | tr -d '"'`
			mode=`curl http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].mode' | tr -d '"'`

			if [ $address == "tcp://"$3":9502" ]; then
				if [ "$mode" == "$4" ]; then
					passed=`expr $passed + 1`
				fi
			fi
			if [ $address == "tcp://"$5":9502" ]; then
				if [ "$mode" == "$6" ]; then
					passed=`expr $passed + 1`
				fi
			fi
			rep_index=`expr $rep_index + 1`
		done
		if [ "$passed" == "$1" ]; then
			echo $2 " passed"
			return
		fi

		i=`expr $i + 1`
		sleep 4
	done
	echo $2 " failed"
	collect_logs_and_exit
}

verify_controller_rep_state() {
	i=0
	rep_state=""
	while [ "$i" != 10 ]; do
		date
		rep_cnt=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].replicaCount'`
		replica_cnt=`expr $rep_cnt`
		rep_index=0
		while [ $rep_index -lt $replica_cnt ]; do
			address=`curl http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].address' | tr -d '"'`
			mode=`curl http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].mode' | tr -d '"'`

			if [ $address == "tcp://"$1":9502" ]; then
				if [ "$mode" == "$2" ]; then
					echo $3" passed"
					return
				fi
				break
			fi
			rep_index=`expr $rep_index + 1`
		done
		i=`expr $i + 1`
		sleep 4
	done
	echo $3 " failed"
	collect_logs_and_exit
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
	verify_replica_cnt "1" "Single replica count test"

	docker stop $replica1_id
	sleep 5

	verify_vol_status "RO" "Single replica stop test"

	docker start $replica1_id

	verify_vol_status "RW" "Single replica start test"
	verify_replica_cnt "1" "Single replica count test"
	verify_controller_rep_state "$REPLICA_IP1" "RW" "Single replica status during start test"
}

test_two_replica_stop_start() {
	verify_replica_cnt "2" "Two replica count test1"

	docker stop $replica1_id
	verify_replica_cnt "1" "Two replica count test when one is stopped"
	verify_vol_status "RO" "when there are 2 replicas and one is stopped"
	verify_controller_rep_state "$REPLICA_IP2" "RW" "Replica2 status after stopping replica1 in 2 replicas case"

	docker start $replica1_id
	verify_replica_cnt "2" "Two replica count test2"
	verify_vol_status "RW" "when there are 2 replicas and one is restarted"

	count=0
	while [ "$count" != 10 ]; do
		docker stop $replica1_id

		docker start $replica1_id &
		sleep `echo "$count * 0.2" | bc`
		docker stop $replica2_id
		# Replica1 might be in Registering mode with status as 'closed' or its rebuild is done with mode as 'RW'
		verify_rep_state 1 "Replica1 status after restarting it, and stopping other one in 2 replicas case" "$REPLICA_IP1" "RW"

		docker start $replica2_id
		verify_replica_cnt "2" "Two replica count test3"
		verify_vol_status "RW" "when there are 2 replicas and replicas restarted multiple times"

		count=`expr $count + 1`
	done


#	verify_controller_rep_state "$REPLICA_IP1" "WO" "Replica1 status after restarting it, and stopping other one in 2 replicas case"

#	verify_vol_status "RO" "restarting stopped replica, and stopped other one in 2 replica case"
#	verify_replica_cnt "1" "Two replica count test when one is restarted and other is stopped"
#	verify_controller_rep_state "$REPLICA_IP1" "RW" "Replica1 status after restarting it in 2 replicas case" -- needed this when replica2 is not stopped

	docker stop $replica1_id
	docker stop $replica2_id
	verify_vol_status "RO" "when there are 2 replicas and both are stopped"
	verify_replica_cnt "0" "Two replica count test when both are stopped"

	docker start $replica1_id
	verify_vol_status "RO" "when there are 2 replicas and are brought down. Then, only one started"
	verify_rep_state 1 "Replica1 status after stopping both, and starting it" "$REPLICA_IP1" "NA"

	docker start $replica2_id
	verify_vol_status "RW" "when there are 2 replicas and are brought down. Then, both are started"
	verify_replica_cnt "2" "when there are 2 replicas and are brought down. Then, both are started"
}

run_ios_to_test_stop_start() {
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 2
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		# Add 4 sec delay in serving IOs from replica1, start IOs, and then close replica1
		# This will trigger the quorum condition which checks if the IOs are
		# written to more than 50% of the replicas

		dd if=/dev/urandom of=/dev/$device_name bs=4k count=1000
		if [ $? -eq 0 ]; then echo "IOs were written successfully while running 3 replicas stop/start test"
		else
			echo "IOs errored out while running 3 replicas stop/start test"; collect_logs_and_exit
		fi
		logout_of_volume
		sleep 5
	else
		echo "Unable to detect iSCSI device, login failed"; collect_logs_and_exit
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
		collect_logs_and_exit
	fi
	docker stop $replica2_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and two are stopped"
	else
		echo "stop/start test failed when there are 3 replicas and two are stopped"
		collect_logs_and_exit
	fi

	docker stop $replica3_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and all are stopped"
	else
		echo "stop/start test failed when there are 3 replicas and all are stopped"
		collect_logs_and_exit
	fi

	docker start $replica1_id
	if [ $(verify_rw_status "RO") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and one is restarted"
	else
		echo "stop/start test failed when there are 3 replicas and one is restarted"
		collect_logs_and_exit
	fi

	docker start $replica2_id
	if [ $(verify_rw_status "RW") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and two are restarted"
	else
		echo "stop/start test failed when there are 3 replicas and two are restarted"
		collect_logs_and_exit
	fi

	docker start $replica3_id
	if [ $(verify_rw_status "RW") == 0 ]; then
		echo "stop/start test passed when there are 3 replicas and all are restarted"
	else
		echo "stop/start test failed when there are 3 replicas and all are restarted"
		collect_logs_and_exit
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

test_replica_reregistration() {
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
			echo "VDbench Test: FAILED";collect_logs_and_exit
		fi
		umount /mnt/store
	else
		echo "Unable to detect iSCSI device, login failed"; collect_logs_and_exit
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
		echo "iSCSI Compliance test: FAILED"; collect_logs_and_exit
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
			echo "DI Test: FAILED"; collect_logs_and_exit
		fi

		cd /mnt/store; sync; sleep 5; sync; sleep 5; cd ~;
		blockdev --flushbufs /dev/$device_name
		hdparm -F /dev/$device_name
		umount /mnt/store
		logout_of_volume
		sleep 5
	else
		echo "Unable to detect iSCSI device, login failed"; collect_logs_and_exit
	fi
}

create_snapshot() {
	id=`curl http://$1:9501/v1/volumes | jq '.data[0].id' |  tr -d '"'`
	curl -H "Content-Type: application/json" -X POST -d '{"name":"snap1"}' http://$CONTROLLER_IP:9501/v1/volumes/$id?action=snapshot
}

test_clone_feature() {
	cloned_controller_id=$(start_controller "$CLONED_CONTROLLER_IP" "store2")
	start_cloned_replica "$CONTROLLER_IP"  "$CLONED_CONTROLLER_IP" "$CLONED_REPLICA_IP" "vol4"

	if [ $(verify_clone_status "completed") == "0" ]; then
		echo "clone created successfully"
	else
		echo "Clone creation failed"
		collect_logs_and_exit
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
			echo "DI Test: FAILED"; collect_logs_and_exit
		fi
	else
		echo "Unable to detect iSCSI device, login failed"; collect_logs_and_exit
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
orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1")
replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
sleep 5
test_single_replica_stop_start
sleep 5
replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
sleep 5
test_two_replica_stop_start
collect_logs_and_exit
sleep 5
replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")
sleep 5
test_three_replica_stop_start
sleep 5
test_replica_reregistration
sleep 5
run_data_integrity_test
sleep 5
create_snapshot "$CONTROLLER_IP"
sleep 5
test_clone_feature
sleep 5
run_vdbench_test_on_volume
sleep 5
run_libiscsi_test_suite

