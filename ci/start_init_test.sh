#!/bin/bash
set -x
CONTROLLER_IP="172.18.0.2"
REPLICA_IP1="172.18.0.3"
REPLICA_IP2="172.18.0.4"
REPLICA_IP3="172.18.0.5"
CLONED_CONTROLLER_IP="172.18.0.6"
CLONED_REPLICA_IP="172.18.0.7"

collect_logs_and_exit() {
	echo "--------------------------docker ps -a-------------------------------------"
	docker ps -a

	echo "--------------------------CONTROLLER REST output---------------------------"
	curl http://$CONTROLLER_IP:9501/v1/volumes | jq
	curl http://$CONTROLLER_IP:9501/v1/replicas | jq
	echo "--------------------------REPLICA 1 LOGS ----------------------------------"
	curl http://$REPLICA_IP1:9502/v1/replicas | jq
	echo "--------------------------REPLICA 2 LOGS ----------------------------------"
	curl http://$REPLICA_IP2:9502/v1/replicas | jq
	echo "--------------------------REPLICA 3  LOGS ---------------------------------"
	curl http://$REPLICA_IP3:9502/v1/replicas | jq

	#Take system output
	ps -auxwww
	top -n 10 -b
	netstat -nap

#	i=0
#	while [ "$i" != 10 ]; do
#		i=`expr $i + 1`
#		echo "CONTROLLER TRACE>>"
#		curl http://$CONTROLLER_IP:9501/debug/pprof/goroutine?debug=2
#		echo "REPLICA 1 TRACE>>"
#		curl http://$REPLICA_IP1:9502/debug/pprof/goroutine?debug=2
#		echo "REPLICA 2 TRACE>>"
#		curl http://$REPLICA_IP2:9502/debug/pprof/goroutine?debug=2
#		echo "REPLICA 3 TRACE>>"
#		curl http://$REPLICA_IP3:9502/debug/pprof/goroutine?debug=2
#		sleep 5
#	done

	echo "ls VOL1>>"
	ls -ltr /tmp/vol1/
	echo "ls VOL2>>"
	ls -ltr /tmp/vol2/
	echo "ls VOL3>>"
	ls -ltr /tmp/vol3/
	#Below is to get stack traces of longhorn processes
	kill -SIGABRT $(ps -auxwww | grep -w longhorn | grep -v grep | awk '{print $2}')

	echo "--------------------------ORIGINAL CONTROLLER LOGS ------------------------"
	docker logs $orig_controller_id
	echo "--------------------------REPLICA 1 LOGS ----------------------------------"
	docker logs $replica1_id
	echo "--------------------------REPLICA 2 LOGS ----------------------------------"
	docker logs $replica2_id
	echo "--------------------------REPLICA 3  LOGS ---------------------------------"
	docker logs $replica3_id
	echo "--------------------------CLONED CONTROLLER LOGS --------------------------"
	docker logs $cloned_controller_id
	echo "--------------------------CLONED REPLICA LOGS -----------------------------"
	docker logs $cloned_replica_id
	exit 1
}
cleanup() {
	rm -rf /tmp/vol*
	rm -rf /mnt/logs
	docker stop $(docker ps -aq)
	docker rm $(docker ps -aq)
}

prepare_test_env() {
	echo "-------------------Prepare test env------------------------"
	cleanup

	mkdir -p /tmp/vol1 /tmp/vol2 /tmp/vol3 /tmp/vol4
	mkdir -p /mnt/store /mnt/store2

	docker network create --subnet=172.18.0.0/16 stg-net
	JI=$(docker images | grep openebs/jiva | awk '{print $1":"$2}' | awk 'NR == 2 {print}')
	JI_DEBUG=$(docker images | grep openebs/jiva | awk '{print $1":"$2}' | awk 'NR == 1 {print}')
	echo "Run CI tests on $JI and $JI_DEBUG"
}

verify_replica_cnt() {
	i=0
	replica_cnt=""
	while [ "$replica_cnt" != "$1" ]; do
		date
		replica_cnt=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].replicaCount'`
		i=`expr $i + 1`
		if [ "$i" == 100 ]; then
			echo $2 " -- failed"
			collect_logs_and_exit
		fi
		sleep 2
	done
	echo $2 " -- passed"
	return
}

# RW=1 RO=0
# verify_rw_status "RO/RW"
verify_rw_status() {
	i=0
	rw_status=""
	while [ "$rw_status" != "$1" ]; do
		ro_status=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].readOnly' | tr -d '"'`
		if [ "$ro_status" == "true" ]; then
			rw_status="RO"
		elif [ "$ro_status" == "false" ]; then
			rw_status="RW"
		fi
		i=`expr $i + 1`
		if [ "$i" == 50 ]; then
			echo "1"
			return
		fi
		sleep 2
	done
	echo "0"
}

verify_rw_rep_count() {
       i=0
       count=""
       while [ "$count" != "$1" ]; do
               count=`get_rw_rep_count`
               i=`expr $i + 1`
               if [ "$i" == 50 ]; then
                       echo "1"
                       return
               fi
               sleep 2
       done
       echo "0"
}

#returns number of replicas connected to controller in RW mode
get_rw_rep_count() {
	rep_index=0
	rw_count=0
	rep_cnt=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].replicaCount'`
	replica_cnt=`expr $rep_cnt`
	while [ $rep_index -lt $replica_cnt ]; do
		mode=`curl http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].mode' | tr -d '"'`
		if [ "$mode" == "RW" ]; then
			rw_count=`expr $rw_count + 1`
		fi
		rep_index=`expr $rep_index + 1`
	done
	echo "$rw_count"
}

#$1 - replication factor
#$2 - message
#this fn checks
   # RW replica count connected to controller
   # consistency factor
   # and RO state of controller
#and verifies whether they are in sync
verify_controller_quorum() {
	i=0
	cf=`expr $1 / 2`
	cf=`expr $cf + 1`
	while [ "$i" != 5 ]; do
		date
		rw_count=$(get_rw_rep_count)
		ro_status=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].readOnly' | tr -d '"'`
		# volume RO status is true
		if [ "$ro_status" == "true" ]; then
			# CF is not met
			if [ "$rw_count" -lt "$cf" ]; then
				echo $2 " -- passed1"
			else
				# if CF is met, volume should be in rw
				ro_status=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].readOnly' | tr -d '"'`
				if [ "$ro_status" == "false" ]; then
					echo $2 " -- passed2"
				else
				# CF is met, and volume is in RO
					echo $2 " -- failed1"
					collect_logs_and_exit
				fi
			fi
		else
			# volume RO status is false
			# CF is met
			if [ "$rw_count" -ge "$cf" ]; then
				echo $2 " -- passed3"
			else
				# if CF is not met, volume should be in RO
				ro_status=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].readOnly' | tr -d '"'`
				if [ "$ro_status" == "true" ]; then
					echo $2 " -- passed4"
				else
				# CF is not met, and volume is in RW
					echo $2 " -- failed2"
					collect_logs_and_exit
				fi
			fi
		fi
		sleep 2
		i=`expr $i + 1`
	done
}
# This verifies the goroutine leaks which happens when a request is made to
# replica_ip:9503.
verify_go_routine_leak() {
    i=0
    date
    no_of_goroutine=`curl http://$2:9502/debug/pprof/goroutine?debug=1 | grep goroutine | awk '{ print $4}'`
    passed=0
    req_cnt=0
    while [ "$i" != 30 ]; do
            curl http://$2:9503 &
            i=`expr $i + 1`
            sleep 2
    done
    wait
    new_no_of_goroutine=`curl http://$2:9502/debug/pprof/goroutine?debug=1 | grep goroutine | awk '{ print $4}'`
    old=`expr $no_of_goroutine + 3`
    if [ $new_no_of_goroutine -lt $old ]; then
             echo $1 --passed
             return
    fi
    echo $1 " -- failed"
    collect_logs_and_exit
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
		if [ "$i" == 100 ]; then
			echo $2 " -- failed"
			collect_logs_and_exit
		fi
		sleep 2
	done
	echo $2 " -- passed"
	return
}

#$1 - pass count
#$2 - message
#$3 - Replica IP
#$4 - its to be mode which will be verified by querying controller
#$5 - another replica IP
#$6 - its to be mode which will be verified by querying controller
#this fn verifies that
    #the state of replica to be in 'closed' state by querying replica (or)
    #the mode to be connected to controller by querying controller
#this fn considered as 'pass' if the result matches with the pass count.
#this fn takes care of checking for two replicas, and thus, pass count is passed by caller
verify_rep_state() {
	i=0
	rep_state=""
	while [ "$i" != 50 ]; do
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
			echo $2 " -- passed"
			return
		fi

		i=`expr $i + 1`
		sleep 2
	done
	echo $2 " -- failed"
	collect_logs_and_exit
}

verify_controller_rep_state() {
	i=0
	rep_state=""
	while [ "$i" != 50 ]; do
		date
		rep_cnt=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].replicaCount'`
		replica_cnt=`expr $rep_cnt`
		rep_index=0
		while [ $rep_index -lt $replica_cnt ]; do
			address=`curl http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].address' | tr -d '"'`
			mode=`curl http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].mode' | tr -d '"'`

			if [ $address == "tcp://"$1":9502" ]; then
				if [ "$mode" == "$2" ]; then
					echo $3" -- passed"
					return
				fi
				break
			fi
			rep_index=`expr $rep_index + 1`
		done
		i=`expr $i + 1`
		sleep 2
	done
	echo $3 " -- failed"
	collect_logs_and_exit
}

# start_controller CONTROLLER_IP
start_controller() {
	controller_id=$(docker run -d --net stg-net --ip $1 -P --expose 3260 --expose 9501 --expose 9502-9504 $JI \
			env REPLICATION_FACTOR="$3" launch controller --frontend gotgt --frontendIP "$1" "$2")
	echo "$controller_id"
}

# start_replica CONTROLLER_IP REPLICA_IP folder_name
start_replica() {
	replica_id=$(docker run -d -it --net stg-net --ip "$2" -P --expose 9502-9504 -v /tmp/"$3":/"$3" $JI \
		launch replica --frontendIP "$1" --listen "$2":9502 --size 2g /"$3")
	echo "$replica_id"
}

# start_controller CONTROLLER_IP (debug build)
start_debug_controller() {
	controller_id=$(docker run -d --net stg-net --ip $1 -P --expose 3260 --expose 9501 --expose 9502-9504 $JI_DEBUG \
			env REPLICATION_FACTOR="$3" DEBUG_TIMEOUT="5" launch controller --frontend gotgt --frontendIP "$1" "$2")
	echo "$controller_id"
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

#verify_delete_replica_unsuccess verifies that when RF condition is not met
#the replicas will not be deleted and error will be returned that replica
#count is not equal to the RF.
verify_delete_replica_unsuccess() {
    expected_error="Error deleting replica" 
    error=$(curl -X "POST" http://$CONTROLLER_IP:9501/v1/delete | jq '.replicas[0].msg' | tr -d '"')
    if [ "$error" != "$expected_error" ]; then
               echo $2"  --failed"
        collect_logs_and_exit
    fi
    #verify whether number of replicas are still the same as it was sent or nor.
    verify_replica_cnt "$1" "$2"
    echo $2"  --passed"
    return
}

#verify_delete_replica verifies that if the replication factor condition
#is met then it will delete the replicas. So before calling this function
#ensure that number of replicas should be equal to the RF.
verify_delete_replica() {
    old_replica_count=$(get_replica_count $CONTROLLER_IP)
    echo "$old_replica_count"
    curl -X "POST" http://$CONTROLLER_IP:9501/v1/delete | jq
    new_replica_count=$(get_replica_count $CONTROLLER_IP)
    echo "$new_replica_count"
    verify_replica_cnt "0" "Zero replica count test"
}

test_two_replica_delete() {
	echo "----------------Test_two_replica_delete--------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	sleep 5
	verify_replica_cnt "2" "Two replica count test1"
	# This will delay sync between replicas
	run_ios_to_test_stop_start
	verify_delete_replica "Delete replicas test2"

	docker stop $replica1_id
	docker stop $replica2_id
	sleep 5

	docker start $replica1_id
	docker start $replica2_id
	sleep 5
	verify_replica_cnt "2" "Two replica count test3"

	docker stop $replica1_id
	verify_replica_cnt "1" "One replica count test4"
	verify_delete_replica_unsuccess "1" "Delete replicas with RF=2 and 1 registered replica test5"

	docker stop $replica2_id
	docker stop $orig_controller_id
	cleanup
}


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

# This will start a controller with debug build which delays the registration
# process of replica and verifies if in case a replica goes down after sending
# request for registeration to controller, controller should send 'start' signal
# to other replica after verifying the replication factor.
test_replica_ip_change() {
	echo "----------------Test_replica_ip_change---------------"
	debug_controller_id=$(start_debug_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2"
	sleep 1

	echo "Stopping replica with IP: $REPLICA_IP1"
	# Injected the delay in sending 'start' signal in the debug_controller
	# and hence crash the replica before getting 'start' signal.
	docker stop $replica1_id
	sleep 3

	curl -k --data "{ \"timeout\":\"0\" }" -H "Content-Type:application/json" -XPOST $CONTROLLER_IP:9501/timeout
	echo "Starting another replica with different IP: $REPLICA_IP3"
	# start the other replica and wait for any one of the two replicas to be
	# registered and get 'start' signal.
	start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3"
	sleep 5

	verify_replica_cnt "2" "Two replica count test1"
	cleanup
}

test_two_replica_stop_start() {
	echo "----------------Test_two_replica_stop_start---------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	sleep 5

	verify_replica_cnt "2" "Two replica count test1"
	# This will delay sync between replicas
	run_ios_to_test_stop_start

	docker stop $replica1_id
	verify_replica_cnt "1" "Two replica count test when one is stopped"
	verify_vol_status "RO" "when there are 2 replicas and one is stopped"
	verify_controller_rep_state "$REPLICA_IP2" "RW" "Replica2 status after stopping replica1 in 2 replicas case"

	docker start $replica1_id
	verify_replica_cnt "2" "Two replica count test2"

	verify_controller_quorum "2" "when there are 2 replicas and one is restarted"
	verify_vol_status "RW" "when there are 2 replicas and one is restarted"
	verify_go_routine_leak "when there are 2 replicas, and sending curl request on data address to" "$REPLICA_IP1"

	count=0
	while [ "$count" != 5 ]; do
		docker stop $replica1_id

		docker start $replica1_id &
		sleep `echo "$count * 0.3" | bc`
		docker stop $replica2_id
		# Replica1 might be in Registering mode with status as 'closed' or its rebuild is done with mode as 'RW'
		verify_rep_state 1 "Replica1 status after restarting it, and stopping other one in 2 replicas case" "$REPLICA_IP1" "RW"

		docker start $replica2_id
		verify_replica_cnt "2" "Two replica count test3"
		verify_vol_status "RW" "when there are 2 replicas and replicas restarted multiple times"

		count=`expr $count + 1`
	done
	verify_controller_quorum "2" "when there are 2 replicas and they are restarted multiple times"
	verify_vol_status "RW" "when there are 2 replicas and they are restarted multiple times"

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

	reader_exit=`docker logs $orig_controller_id 2>&1 | grep "Exiting rpc reader" | wc -l`
	writer_exit=`docker logs $orig_controller_id 2>&1 | grep "Exiting rpc writer" | wc -l`
	loop_exit=`docker logs $orig_controller_id 2>&1 | grep "Exiting rpc loop" | wc -l`
	if [ "$reader_exit" == 0 ]; then
		collect_logs_and_exit
	fi
	if [ "$writer_exit" == 0 ]; then
		collect_logs_and_exit
	fi
	if [ "$loop_exit" == 0 ]; then
		collect_logs_and_exit
	fi

	cleanup
}

run_ios_to_test_stop_start() {
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 2
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		# Add 4 sec delay in serving IOs from replica1, start IOs, and then close replica1
		# This will trigger the quorum condition which checks if the IOs are
		# written to more than 50% of the replicas

		dd if=/dev/urandom of=/dev/$device_name bs=4k count=50000
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
	echo "-----------------Test_three_replica_stop_start---------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")

	sleep 5

	count=0
	while [ "$count" != 5 ]; do
		docker stop $orig_controller_id &
		docker stop $replica1_id &
		wait
		sleep 5
		docker start $orig_controller_id
		docker start $replica1_id
		verify_replica_cnt "3" "Three replica count test when controller restarted multiple times"
		verify_vol_status "RW" "when there are 3 replicas and controller restarted multiple times"
		count=`expr $count + 1`
	done

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
		if [ $i -eq 50 ]; then
			echo "Closed replica failed to attach back to controller"
			exit;
		fi
		echo "Wait for the closed replica to connect back to controller, replicaCount: "$replica_count
		sleep 5;
		replica_count=$(get_replica_count $CONTROLLER_IP)
	done

	wait
	cleanup
}

test_ctrl_stop_start() {
       echo "-----------------Test_three_replica_stop_start---------------"
       orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
       replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
       replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
       replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")

       if [ $(verify_rw_rep_count "3") != 0 ]; then
               echo "test_ctrl_stop_start() Verify_rw_rep_count failed"
               collect_logs_and_exit
       fi

       docker stop $orig_controller_id
       docker start $orig_controller_id

       if [ $(verify_rw_rep_count "3") != 0 ]; then
               echo "test_ctrl_stop_start() Verify_rw_rep_count failed"
               collect_logs_and_exit
       fi

       cleanup
}

test_replica_reregistration() {
	echo "----------------Test_replica_reregistration------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")
	sleep 5
	i=0
	replica_count=$(get_replica_count $CONTROLLER_IP)
	while [ "$replica_count" != 3 ]; do
		i=`expr $i + 1`
		if [ $i -eq 50 ]; then
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
		if [ $i -eq 50 ]; then
			echo "Closed replica failed to attach back to controller"
			exit;
		fi
		echo "Wait for the closed replica to connect back to controller, replicaCount: "$replica_count
		sleep 5;
		replica_count=$(get_replica_count $CONTROLLER_IP)
	done
	cleanup
}

run_vdbench_test_on_volume() {
	echo "-----------------Run_vdbench_test_on_volume------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")

	sleep 5
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 5
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		mkfs.ext2 -F /dev/$device_name
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
	cleanup
}

run_libiscsi_test_suite() {
	echo "----------------Run_libiscsi_test_suite---------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")

	sleep 5
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
	cleanup
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
	echo "--------------------Run_data_integrity_test------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")

	sleep 5
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
	#Cleanup is not being performed here because this data will be used to test clone feature in the next test
}

create_snapshot() {
	echo "--------------create_snapshot-------------"
	id=`curl http://$1:9501/v1/volumes | jq '.data[0].id' |  tr -d '"'`
	curl -H "Content-Type: application/json" -X POST -d '{"name":"snap1"}' http://$CONTROLLER_IP:9501/v1/volumes/$id?action=snapshot
}

test_clone_feature() {
	echo "-----------------------Test_clone_feature-------------------------"
	cloned_controller_id=$(start_controller "$CLONED_CONTROLLER_IP" "store2" "1")
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
	cleanup
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

# test_extent_support_file_system tests whether the file system supports
# extent mapping. If it doesnot replica will error out.
# Creating a file system of FAT type which doesn't support extent mapping.
test_extent_support_file_system() {
	echo "-----------Run_extent_supported_file_system_test-------------"
	mkdir -p /tmp/vol1
	# create a file
	truncate -s 2100M testfat
	# losetup is used to associate block device
	# get the free device
	device=$(sudo losetup -f)
	# attach the loopback device with regular disk file testfat
	losetup $device testfat
	# create a FAT file system
	mkfs.fat testfat
	# mount as a block device in /tmp/vol1
	mount $device /tmp/vol1

	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "1")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	sleep 5

	verify_replica_cnt "0" "Zero replica count test1"

	error=$(docker logs $replica1_id 2>&1 | grep -w "underlying file system does not support extent mapping")
	count=$(echo $error | wc -l)

	if [ "$count" -eq 0  ]; then
		echo "extent supported file system test failed"
		umount /tmp/vol1
		losetup -d $device
		collect_logs_and_exit
	else
		echo "extent support file system test --passed"
	fi
	umount /tmp/vol1
	losetup -d $device
	cleanup
}


prepare_test_env
test_single_replica_stop_start
test_two_replica_delete
test_replica_ip_change
test_two_replica_stop_start
test_three_replica_stop_start
test_ctrl_stop_start
test_replica_reregistration
run_data_integrity_test
create_snapshot "$CONTROLLER_IP"
test_clone_feature
test_extent_support_file_system
run_vdbench_test_on_volume
run_libiscsi_test_suite
