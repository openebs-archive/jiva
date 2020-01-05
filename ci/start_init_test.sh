#!/bin/bash
#set -x
PS4='${LINENO}: '
CONTROLLER_IP="172.18.0.2"
REPLICA_IP1="172.18.0.3"
REPLICA_IP2="172.18.0.4"
REPLICA_IP3="172.18.0.5"
CLONED_CONTROLLER_IP="172.18.0.6"
CLONED_REPLICA_IP="172.18.0.7"

snapIndx=1

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
  cat $(ls /tmp/vol1/*.meta)
	echo "ls VOL2>>"
	ls -ltr /tmp/vol2/
  cat $(ls /tmp/vol2/*.meta)
	echo "ls VOL3>>"
	ls -ltr /tmp/vol3/
  cat $(ls /tmp/vol3/*.meta)
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

collect_logs() {
        echo "--------------------------Collect logs----------------------------"
	echo "--------------------------ORIGINAL CONTROLLER LOGS ------------------------"
	docker logs $orig_controller_id
	echo "--------------------------DEBUG REPLICA 1 LOGS-----------------------------------"
	docker logs $replica1_id
	echo "--------------------------DEBUG REPLICA 2 LOGS-----------------------------------"
	docker logs $replica2_id

}

cleanup() {
        echo "----------------------------Cleanup------------------------"
	rm -rfv /tmp/vol*
	rm -rfv /mnt/logs
	docker stop $(docker ps -aq)
	docker rm $(docker ps -aq)
}

prepare_test_env() {
	echo "-------------------Prepare test env------------------------"
	cleanup

	mkdir -pv /tmp/vol1 /tmp/vol2 /tmp/vol3 /tmp/vol4
	mkdir -pv /mnt/store /mnt/store2

	docker network create --subnet=172.18.0.0/16 stg-net
	JI=$(docker images | grep openebs/jiva | awk '{print $1":"$2}' | awk 'NR == 2 {print}')
	JI_DEBUG=$(docker images | grep openebs/jiva | awk '{print $1":"$2}' | awk 'NR == 1 {print}')
	echo "Run CI tests on $JI and $JI_DEBUG"
}

verify_replica_cnt() {
        echo "-------------------Verify Replica count--------------------"
	i=0
	replica_cnt=""
	while [ "$replica_cnt" != "$1" ]; do
		replica_cnt=`curl -s http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].replicaCount'`
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

verify_container_dead() {
	i=0
	replica_id=`echo $1 | cut -b 1-12`
	echo $replica_id
	while [ $i -lt 100 ]; do
		cnt=`docker ps -a | grep $replica_id | grep -w Restarting | wc -l`
		if [ $cnt -eq 1 ]; then
			return
		fi
		sleep 2
		i=`expr $i + 1`
	done
	docker ps -a
	echo $1
	echo $2 " -- failed"
	collect_logs_and_exit
}

# RW=1 RO=0
# verify_rw_status "RO/RW"
verify_rw_status() {
	i=0
	rw_status=""
	while [ "$rw_status" != "$1" ]; do
		ro_status=`curl -s http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].readOnly' | tr -d '"'`
		if [ "$ro_status" == "true" ]; then
			rw_status="RO"
		elif [ "$ro_status" == "false" ]; then
			rw_status="RW"
		fi
		i=`expr $i + 1`
		if [ "$i" == 50 ]; then
			echo "verify_rw_status -- failed"
			collect_logs_and_exit
		fi
		sleep 2
	done
	echo "0"
}

verify_rw_rep_count() {
        echo "---------------Verify RW Replica Status------------------"
       i=0
       count=""
       while [ "$count" != "$1" ]; do
               count=`get_rw_rep_count`
               i=`expr $i + 1`
               if [ "$i" == 100 ]; then
		       echo "verify_rw_rep_count -- failed"
		       collect_logs_and_exit
               fi
               sleep 2
       done
       echo "Verify RW Replica status test -- passed"
}

#returns number of replicas connected to controller in RW mode
get_rw_rep_count() {
	rep_index=0
	rw_count=0
	rep_cnt=`curl -s http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].replicaCount'`
	replica_cnt=`expr $rep_cnt`
	while [ $rep_index -lt $replica_cnt ]; do
		mode=`curl -s http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].mode' | tr -d '"'`
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
        echo "--------------------Verify controller quorum-----------------"
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
    echo "---------------------Verify goroutine leak-------------------------"
    i=0
    no_of_goroutine=`curl http://$2:9502/debug/pprof/goroutine?debug=1 | grep goroutine | awk '{ print $4}'`
    passed=0
    req_cnt=0
    while [ "$i" != 30 ]; do
            curl -m 5 -s http://$2:9503 &
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
        echo "---------------------------Verify volume status--------------------------"
	i=0
	rw_status=""
	while [ "$rw_status" != "$1" ]; do
		ro_status=`curl -s http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].readOnly' | tr -d '"'`
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

verify_replica_mode() {
	i=0
        passed=0
	while [ "$i" != 50 ]; do
		date
		rep_mode=`curl -s http://$3:9502/v1/replicas | jq '.data[0].replicamode' | tr -d '"'`
		if [ "$rep_mode" == "$4" ]; then
			passed=`expr $passed + 1`
		fi
		if [ "$passed" == "$1" ]; then
			echo $2 " -- passed"
			return
		fi
	done
	echo $2 " -- failed"
	collect_logs_and_exit
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
        echo "------------------------Verify Replica state--------------------------"
	i=0
	rep_state=""
	while [ "$i" != 50 ]; do
		date
		rep_cnt=`curl -s http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].replicaCount'`
		replica_cnt=`expr $rep_cnt`
		passed=0
		#if [ "$replica_cnt" == 0 ]; then
			rep_state=`curl -s http://$3:9502/v1/replicas | jq '.data[0].state' | tr -d '"'`
			if [ "$rep_state" == "closed" ]; then
				passed=`expr $passed + 1`
			fi
			if [ "$5" != "" ]; then
				rep_state=`curl -s http://$5:9502/v1/replicas | jq '.data[0].state' | tr -d '"'`
				if [ "$rep_state" == "closed" ]; then
					passed=`expr $passed + 1`
				fi
			fi
		#fi
		rep_index=0
		while [ $rep_index -lt $replica_cnt ]; do
			address=`curl -s http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].address' | tr -d '"'`
			mode=`curl -s http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].mode' | tr -d '"'`

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
        echo "--------------------Verify controller and replica state------------------"
	i=0
	rep_state=""
	while [ "$i" != 50 ]; do
		date
		rep_cnt=`curl -s http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].replicaCount'`
		replica_cnt=`expr $rep_cnt`
		rep_index=0
		while [ $rep_index -lt $replica_cnt ]; do
			address=`curl -s http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].address' | tr -d '"'`
			mode=`curl -s http://$CONTROLLER_IP:9501/v1/replicas | jq '.data['$rep_index'].mode' | tr -d '"'`

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
	replica_id=$(docker run --restart unless-stopped -d --net stg-net --ip "$2" -P --expose 9502-9504 -v /tmp/"$3":/"$3" $JI \
		launch replica --frontendIP "$1" --listen "$2":9502 --size 2g /"$3")
	echo "$replica_id"
}

# start_replica CONTROLLER_IP REPLICA_IP folder_name
start_debug_replica() {
	replica_id=$(docker run --restart unless-stopped -d --net stg-net --ip "$2" -P --expose 9502-9504 -v /tmp/"$3":/"$3" $JI_DEBUG \
	env $4=$5  $6=$7 launch replica --frontendIP "$1" --listen "$2":9502 --size 2g /"$3")
	echo "$replica_id"
}

# start_controller CONTROLLER_IP (debug build)
start_debug_controller() {
	controller_id=$(docker run -d --net stg-net --ip $1 -P --expose 3260 --expose 9501 --expose 9502-9504 $JI_DEBUG \
			env REPLICATION_FACTOR="$3" "$4"="$5" launch controller --frontend gotgt --frontendIP "$1" "$2")
	echo "$controller_id"
}

# start_cloned_replica CONTROLLER_IP  CLONED_CONTROLLER_IP CLONED_REPLICA_IP folder_name
start_cloned_replica() {
	cloned_replica_id=$(docker run --restart unless-stopped -d --net stg-net --ip "$3" -P --expose 9502-9504 -v /tmp/"$4":/"$4" $JI \
		launch replica --type clone --snapName snap3 --cloneIP "$1" --frontendIP "$2" --listen "$3":9502 --size 2g /"$4")
	echo "$cloned_replica_id"
}

# get_replica_count CONTROLLER_IP
get_replica_count() {
	replicaCount=`curl -s http://"$1":9501/v1/volumes | jq '.data[0].replicaCount'`
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
	echo "-----------------------Test_two_replica_delete-----------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	sleep 5
	verify_replica_cnt "2" "Two replica count test1"
	# This will delay sync between replicas
	run_ios 50K 0
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

test_replication_factor() {
	echo "-------------------------Test_replication_factor---------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "1")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	verify_replica_cnt "1" "Single replica count test"
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	sleep 5

	verify_replica_cnt "1" "Single replica count test"
	add_replica_exit=$(docker logs $orig_controller_id 2>&1 | grep "error: replication factor: 1, added replicas: 1" | wc -l)
	if [ "$add_replica_exit" == 0 ]; then
		collect_logs_and_exit
	fi

	sudo docker stop $replica1_id
	sudo docker start $replica1_id
	sleep 5

	verify_replica_cnt "1" "Single replica count test"
	add_replica_exit=$(docker logs $orig_controller_id 2>&1 | grep "error: replication factor: 1, added replicas: 1" | wc -l)
	if [ "$add_replica_exit" == 0 ]; then
		collect_logs_and_exit
	fi

	echo "test_replication_factor --passed"
	cleanup
}

test_single_replica_stop_start() {
	echo "----------------------Test_single_replica_stop_start-------------------"
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
	echo "-------------------------Test_replica_ip_change------------------------"
	debug_controller_id=$(start_debug_controller "$CONTROLLER_IP" "store1" "2" "DEBUG_TIMEOUT" "5")
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

test_preload() {
	echo "-----------------------------Test_preload-------------------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "1")
	debug_replica_id=$(start_debug_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1" "PRELOAD_TIMEOUT" "12")
        # Since this is first time for replica to connect
        # timeout of 12sec is injected
        sleep 15
	sudo docker stop $orig_controller_id
	sudo docker start $orig_controller_id
	verify_replica_cnt "1" "One replica count test when controller is restarted"

        # Restart controller once it has registered
        # to test whether rpc connection is closed,
        # since tcp connection is created only after
        # replica is registered.
        sudo docker stop $orig_controller_id
	sudo docker start $orig_controller_id
	verify_replica_cnt "1" "One replica count test when controller is restarted"
	sleep 5

	rpc_close=`docker logs $debug_replica_id 2>&1 | grep -c "Closing RPC conn with replica:"`
	if [ "$rpc_close" == 0 ]; then
		collect_logs_and_exit
	fi

	controller_exit=`docker logs $orig_controller_id 2>&1 | grep -c "Stopping controller"`
	if [ "$controller_exit" == 0 ]; then
		collect_logs_and_exit
	fi

	register_replica=`docker logs $debug_replica_id 2>&1 | grep -c "Register replica at controller"`
	if [ "$register_replica" -lt 2 ]; then
		collect_logs_and_exit
	fi


        preload_success=0
        iter=0
	while [ "$preload_success" -lt 1 ]; do
		preload_success=`docker logs $debug_replica_id 2>&1 | grep -c "Read extents successful"`
                if [ "$iter" == 100 ]; then
                        collect_logs_and_exit
                fi
                let iter=iter+1
                sleep 2
        done

	cleanup
}

test_controller_rpc_close() {
	echo "------------------------Test_controller_rpc_close--------------------------"
	debug_controller_id=$(start_debug_controller "$CONTROLLER_IP" "store1" "1" "RPC_PING_TIMEOUT" "45")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
        # Adding this sleep to ensure ping timeout
        sleep 60

        writer_exit=`docker logs $debug_controller_id 2>&1 | grep "Exiting rpc writer" | wc -l`
        reader_exit=`docker logs $debug_controller_id 2>&1 | grep "Exiting rpc reader" | wc -l`
	loop_exit=`docker logs $debug_controller_id 2>&1 | grep "Exiting rpc loop" | wc -l`
	if [ "$writer_exit" == 0 ]; then
		collect_logs_and_exit
	fi
        if [ "$reader_exit" == 0 ]; then
		collect_logs_and_exit
	fi
	if [ "$loop_exit" == 0 ]; then
		collect_logs_and_exit
	fi

	curl -k --data "{ \"rpcPingTimeout\":\"0\" }" -H "Content-Type:application/json" -XPOST $CONTROLLER_IP:9501/timeout
	verify_replica_cnt "1" "One replica count test1"

	cleanup
}

test_replica_rpc_close() {
	echo "-------------------------Test_replica_rpc_close----------------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "1")
	debug_replica_id=$(start_debug_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	sleep 5
        docker stop $orig_controller_id
        sleep 25

	read_write_exit=`docker logs $debug_replica_id 2>&1 | grep -c "Closing RPC conn with replica:"`
        if [ "$read_write_exit" == 0 ]; then
		collect_logs_and_exit
	fi

	cleanup
}

verify_bad_file_descriptor_error() {
	echo "-----------------Verify_bad_file_descriptor_error-------------"
	verify_replica_cnt "2" "Two replica count test"
	# Generate sufficient no of extents so that punch hole get
	# delayed and we can verify that data which is filled in the
	# HoleCreatorChan during preload is drained while closing the replica.
	# Replica will be closed upon controller disconnection
	run_ios_rand_write "1"
	docker stop $orig_controller_id

	verify_container_dead $replica1_id "controller restart in verify_bad_file_descriptor_error test"
	verify_container_dead $replica2_id "controller restart in verify_bad_file_descriptor_error test"

	sleep 5
	docker start $orig_controller_id
	docker start $replica1_id
	docker start $replica2_id
	verify_replica_cnt "2" "Two replica count test when controller is stopped in test_bad_file_descriptor"
	verify_vol_status "RW" "When there are 2 replicas and controller is stopped in test_bad_file_descriptor"


	docker stop $replica1_id
	replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	verify_replica_cnt "2" "Two replica count test when replica is stopped and started non debug mode in test_bad_file_descriptor"
	verify_vol_status "RW" "When there are 2 replicas and debug replica is stopped and started in non debug mode in test_bad_file_descriptor"

	run_ios_rand_write "1" &
	sleep 1
	docker stop $orig_controller_id
	sleep 1
	docker start $orig_controller_id
	sleep 25

	replica1_exit=`docker logs $replica1_id 2>&1 | grep "ERROR in creating hole: bad file descriptor" | wc -l`
	replica2_exit=`docker logs $replica2_id 2>&1 | grep "ERROR in creating hole: bad file descriptor" | wc -l`
	replica3_exit=`docker logs $replica2_id 2>&1 | grep "ERROR in creating hole: bad file descriptor" | wc -l`
	if [ "$replica1_exit" != 0  ] || [ "$replica2_exit" != 0 ] || [ "$replica3_exit" != 0 ]; then
		echo "test_bad_file_descriptor failed " "$1"
		collect_logs
	fi

	wait
	verify_replica_cnt "2" "Two replica count test when controller is stopped in test_bad_file_descriptor"
	verify_vol_status "RW" "When there are 2 replicas and controller is stopped in test_bad_file_descriptor"

	cleanup
}

# Test bad file descriptor verifies whether replica is crashing while punching
# holes for the duplicate data.
test_bad_file_descriptor() {
	echo "----------------Test_bad_file_descripter---------------"
        # Test case1: When punching holes while writing
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_debug_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1" "PUNCH_HOLE_TIMEOUT" "5")
	replica2_id=$(start_debug_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2" "PUNCH_HOLE_TIMEOUT" "5")
	sleep 5

	# Generate sufficient no of extents so that punch hole
	# is called while writing duplicate data and verify that
	# data in HoleCreatorChan during write is drained while
	# closing the replica.
	# Replica will be closed upon controller disconnection
        verify_bad_file_descriptor_error "When punching holes while writing"

        # Test case2: When punching holes while preload
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_debug_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1" "PUNCH_HOLE_TIMEOUT" "5" "DISABLE_PUNCH_HOLES" "True")
	replica2_id=$(start_debug_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2" "PUNCH_HOLE_TIMEOUT" "5" "DISABLE_PUNCH_HOLES" "True")
	sleep 5

	# Generate sufficient no of extents so that punch hole get
	# delayed and we can verify that data which is filled in the
	# HoleCreatorChan during preload is drained while closing the replica.
	# Replica will be closed upon controller disconnection
        verify_bad_file_descriptor_error "When punching holes while preload"
}

test_two_replica_stop_start() {
	echo "------------------------Test_two_replica_stop_start------------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	sleep 5

	verify_replica_cnt "2" "Two replica count test1"
	# This will delay sync between replicas
	run_ios 50K 0

	docker stop $replica1_id
	verify_replica_cnt "1" "Two replica count test when one is stopped"
	verify_vol_status "RO" "When there are 2 replicas and one is stopped"
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

run_ios_rand_write() {
	echo "-------------------------Run IOS (Random write)-------------------------"
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 2
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		i=0
		while [ "$i" != 25 ];
		do
			dd if=/dev/urandom of=/dev/$device_name bs=4K count=$1 seek=`expr $i \* 2`
			if [ $? -ne 0 ];
			then
				echo "IOs errored out while running bad file descriptor test";
				collect_logs_and_exit
			fi
			let i=i+1
		done
		logout_of_volume
		sleep 5
	else
		echo "Unable to detect iSCSI device, login failed";
		collect_logs_and_exit
	fi
}

run_ios() {
        echo "---------------------------Run IO's-------------------------------"
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 2
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		# Add 4 sec delay in serving IOs from replica1, start IOs, and then close replica1
		# This will trigger the quorum condition which checks if the IOs are
		# written to more than 50% of the replicas

		dd if=/dev/urandom of=/dev/$device_name bs=4K count=$1 seek=$2
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

	run_ios 50K 0 &
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
			collect_logs_and_exit
		fi
		echo "Wait for the closed replica to connect back to controller, replicaCount: "$replica_count
		sleep 5;
		replica_count=$(get_replica_count $CONTROLLER_IP)
	done

	wait
	cleanup
}

test_ctrl_stop_start() {
       echo "-----------------Test Controller stop/start---------------"
       orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
       replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
       replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
       replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")

       verify_rw_rep_count "3"

       docker stop $orig_controller_id
       docker start $orig_controller_id

       verify_rw_rep_count "3"
       echo "Test Controller stop/start test --passed"
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
			collect_logs_and_exit
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
			collect_logs_and_exit
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
        echo "-------------------------Get SCSI Disk---------------------------"
	device_name=$(iscsiadm -m session -P 3 |grep -i "Attached scsi disk" | awk '{print $4}')
	i=0
	while [ -z $device_name ]; do
		sleep 5
		device_name=$(iscsiadm -m session -P 3 |grep -i "Attached scsi disk" | awk '{print $4}')
		i=`expr $i + 1`
		if [ $i -eq 10 ]; then
			echo "scsi disk not found";
			collect_logs_and_exit
		else
			continue;
		fi
	done
        echo "SCSI disk:" $device_name
}


# in this test we write some data on the block device
# and then verify whether data saved into the replicas
# are same.
test_data_integrity() {
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 5
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		mkfs.ext2 -F /dev/$device_name

		mount /dev/$device_name /mnt/store

		dd if=/dev/urandom of=file1 bs=4k count=10000
                hash1=$(md5sum file1 | awk '{print $1}')
		cp file1 /mnt/store
		umount /mnt/store
		logout_of_volume
	        login_to_volume "$CONTROLLER_IP:3260"
	        get_scsi_disk
	        sleep 5
		mount /dev/$device_name /mnt/store

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

run_data_integrity_test_with_fs_creation() {
	echo "--------------------Run_data_integrity_test------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")
	sleep 5
	test_data_integrity
	#Cleanup is not being performed here because this data will be used
	#to test snapshot feature in the next test.
	# value of hash1 will be used for clone.
}

# create_and_verify_snapshot creates a snapshot and verifies if it
# has message given below (success case) or not (failure case).
# As a part of negative test we are trying to create the snapshot with the
# same name which returns snapshot_exists.
create_snapshot() {
	message=`curl -H "Content-Type: application/json" -X POST -d '{"name":"'$2'"}' http://$CONTROLLER_IP:9501/v1/volumes/$1?action=snapshot | jq '.message' | tr -d '"'`
	if [ "$message" == "$3" ] ;
	then
		echo "create snapshot test passed"
	else
		echo "create snapshot test failed"
		collect_logs_and_exit
	fi;
}

test_duplicate_snapshot_failure() {
	echo "--------------Test duplicate snapshot Failure-------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	verify_rw_rep_count "2"
	id=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].id' |  tr -d '"'`
	create_snapshot $id "snap1" "Snapshot: snap1 created successfully"
	create_snapshot $id "snap1" "Snapshot: snap1 already exists"
	create_snapshot $id "snap2" "Snapshot: snap2 created successfully"
	sleep 5
	test_data_integrity
	cleanup
}

test_clone_feature() {
	echo "-----------------------Test_clone_feature-------------------------"
	id=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].id' |  tr -d '"'`
	create_snapshot $id "snap3" "Snapshot: snap3 created successfully"
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
		clonestatus=`curl -s http://$CLONED_REPLICA_IP:9502/v1/replicas/1 | jq '.clonestatus' | tr -d '"'`
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

create_device() {
	# losetup is used to associate block device
	# get the free device
	device=$(sudo losetup -f)
	echo $device
}

verify_extent_mapping_support() {
	error=$(docker logs "$1" 2>&1 | grep -w "failed to find extents, error: operation not permitted")
	count=$(echo $error | wc -l)

	if [ "$count" -eq 0  ]; then
		echo "extent supported file system test failed"
		umount /tmp/vol1
		wait
		losetup -d "$2"
		collect_logs_and_exit
	fi
	echo "extent support file system test --passed"
	return
}

# test_extent_support_file_system tests whether the file system supports
# extent mapping. If it doesnot replica will error out.
# Creating a file system of FAT type which doesn't support extent mapping.
test_extent_support_file_system() {
	echo "-----------Run_extent_supported_file_system_test-------------"
	mkdir -p /tmp/vol1 /tmp/vol2
	device1=$(create_device)
	# attach the loopback device with regular disk file testfat1
	truncate -s 2100M testfat1
	losetup $device1 testfat1
	# create a FAT file system
	mkfs.fat testfat1
	mount $device1 /tmp/vol1

	device2=$(create_device)
	truncate -s 2100M testfat2
	losetup $device2 testfat2
	# create a FAT file system
	mkfs.fat testfat2
	mount $device2 /tmp/vol2

	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "1")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	sleep 5

	verify_replica_cnt "0" "Zero replica count test1"
	sudo docker start $replica1_id
	sudo docker start $replica2_id
	sleep 5

	verify_extent_mapping_support "$replica1_id1" "$device1"
	verify_extent_mapping_support "$replica2_id2" "$device2"

	umount /tmp/vol1
	umount /tmp/vol2
	wait
	losetup -d $device1
	losetup -d $device2
	rm -f testfat1 testfat2
	cleanup
}

upgrade_controller() {
       echo "--------------------- Upgrade Controller --------------------"
       docker logs $orig_controller_id
       docker stop $orig_controller_id
       docker rm $orig_controller_id
       orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
}

upgrade_replicas() {
       echo "--------------------- Upgrade Replica --------------------"
       docker logs $replica1_id
       docker stop $replica1_id
       docker rm $replica1_id
       replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
       docker logs $replica2_id
       docker stop $replica2_id
       docker rm $replica2_id
       replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
       docker logs $replica3_id
       docker stop $replica3_id
       docker rm $replica3_id
       replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")
}

test_upgrade() {
	# This test is being performend because there is a change in the
	# data structures used for communication between controller and replica.
	echo "----------------Test_upgrade----------------"
	docker pull $1
	UPGRADED_JI=$JI
	JI=$1

	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")

	verify_replica_cnt "3" "Three replica count test in controller upgrade"
	run_ios 50K 0 &
	sleep 8

	JI=$UPGRADED_JI
	if [ "$2" == "controller-replica" ]; then
		upgrade_controller
		upgrade_replicas
	else
		upgrade_replicas
		upgrade_controller
	fi
	verify_replica_cnt "3" "Three replica count test in controller upgrade"
	wait
	test_data_integrity
	cleanup
}

test_upgrades() {
       test_upgrade "openebs/jiva:1.4.0" "replica-controller"
}

di_test_on_raw_disk() {
	echo "----------------Test_di_on_raw_disk---------------"
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 5
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		# This tests resize feature
		rm test_file1 test_file2
		if [ "$1" != "" ]; then
			dd if=/dev/urandom of=test_file1 bs=4K count=4K
			dd if=test_file1 of=/dev/$device_name bs=4K count=4K seek=$1
			dd if=/dev/$device_name of=test_file2 bs=4K count=4K skip=$1
			hash1=$(md5sum test_file1 | awk '{print $1}')
			hash2=$(md5sum test_file2 | awk '{print $1}')
			if [ $hash1 == $hash2 ]; then echo "DI Test: PASSED"
			else
				echo "DI Test: FAILED"; collect_logs_and_exit
			fi
			logout_of_volume
			sleep 5
			login_to_volume "$CONTROLLER_IP:3260"
			sleep 5
			get_scsi_disk
			dd if=/dev/$device_name of=test_file3 bs=4K count=4K skip=$1
			hash1=$(md5sum test_file1 | awk '{print $1}')
			hash3=$(md5sum test_file3 | awk '{print $1}')
			if [ $hash1 == $hash3 ]; then echo "DI Test: PASSED"
			else
				fdisk -l
				logout_of_volume
				echo "DI Test: FAILED"; collect_logs_and_exit
			fi
			logout_of_volume
		fi
	fi
	echo "Test_di_on_raw_disk passed"
}

test_replica_controller_continuous_stop_start() {
	echo "----------------Test_replica_controller_continuous_stop_start-----------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "1")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	verify_rw_rep_count "1"
	di_test_on_raw_disk "1K"
	sleep 5

	# Test pod restarts on same node
	docker stop $replica1_id
	docker start $replica1_id
	verify_rw_rep_count "1"
	di_test_on_raw_disk "1K"
	docker stop $replica1_id
	docker start $replica1_id
	verify_rw_rep_count "1"
	di_test_on_raw_disk "1K"
	docker stop $orig_controller_id
	docker start $orig_controller_id
	verify_rw_rep_count "1"
	di_test_on_raw_disk "1K"
	cleanup
	echo "Test_replica_controller_continuous_stop_start passed"
}

test_volume_resize() {
	echo "----------------Test_volume_resize---------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	verify_rw_rep_count "2"
	id=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].id' |  tr -d '"'`
	curl -H "Content-Type: application/json" -X POST -d '{"name":"store1", "size": "10G"}' http://$CONTROLLER_IP:9501/v1/volumes/$id?action=resize

	upgraded_size=$((10*1024*1024*1024))
	size=`curl http://$REPLICA_IP1:9502/v1/replicas/1 | jq '.size' | tr -d '"'`
	if [[ $size != $upgraded_size ]]; then
		echo "Resize test failed"
	fi
	size=`curl http://$REPLICA_IP2:9502/v1/replicas/1 | jq '.size' | tr -d '"'`
	if [[ $size != $upgraded_size ]]; then
		echo "Resize test failed"
	fi
	di_test_on_raw_disk "2M"
	sleep 5
	# Test pod restarts on same node
	docker stop $replica1_id
	docker start $replica1_id
	verify_rw_rep_count "2"
	di_test_on_raw_disk "2M"
	docker stop $replica1_id
	docker stop $replica2_id
	docker start $replica1_id
	docker start $replica2_id
	verify_rw_rep_count "2"
	di_test_on_raw_disk "2M"
	docker stop $orig_controller_id
	docker start $orig_controller_id
	verify_rw_rep_count "2"
	di_test_on_raw_disk "2M"
	cleanup
	echo "Resize test passed"
}

create_auto_generated_snapshot() {

	snaplist_initial=`ls /tmp/vol1 | grep .img | grep -v meta | grep  -v head`

	# This will create an auto generated snapshot(Snap1) with the above data
	docker stop $replica1_id
	verify_rw_rep_count "1"
	docker start $replica1_id
	verify_rw_rep_count "2"
	snaplist_final=`ls /tmp/vol1 | grep .img | grep -v meta | grep  -v head`

	snaps[$snapIndx]=`echo ${snaplist_initial[@]} ${snaplist_final[@]} | tr ' ' '\n' | sort | uniq -u`
	let snapIndx=snapIndx+1
	active_file1=`ls /tmp/vol1 | grep .img | grep -v meta | grep head`
	active_file2=`ls /tmp/vol2 | grep .img | grep -v meta | grep head`
	active_file_size=0
}

create_manual_snapshot() {
	snaplist_initial=`ls /tmp/vol1 | grep .img | grep -v meta | grep  -v head`
	id=`curl http://$CONTROLLER_IP:9501/v1/volumes | jq '.data[0].id' |  tr -d '"'`
	create_snapshot $id $1 "Snapshot: $1 created successfully"
	snaplist_final=`ls /tmp/vol1 | grep .img | grep -v meta | grep  -v head`
	snaps[$snapIndx]=`echo ${snaplist_initial[@]} ${snaplist_final[@]} | tr ' ' '\n' | sort | uniq -u`
	let snapIndx=snapIndx+1
	active_file1=`ls /tmp/vol1 | grep .img | grep -v meta | grep head`
	active_file2=`ls /tmp/vol2 | grep .img | grep -v meta | grep head`
	active_file_size=0
}

verify_physical_space_consumed() {
	size=`du -sh --block-size=1048576 /tmp/vol1/$active_file1 | awk '{print $1}'`
	if [ $size != $active_file_size ] && [ $size != `expr $active_file_size + 1` ] ; then
		echo "Active file size check failed for replica1"; collect_logs_and_exit
	fi
	size=`du -sh --block-size=1048576 /tmp/vol2/$active_file2 | awk '{print $1}'`
	if [ $size != $active_file_size ] && [ $size != `expr $active_file_size + 1` ] ; then
		echo "Active file size check failed for replica2"; collect_logs_and_exit
	fi
	physical_size=0
	#for snap in ${snaps[@]}
	for (( i = 1 ; i <= "${#snaps[@]}" ; i++ ))
	do
		size=`du -sh --block-size=1048576 /tmp/vol1/${snaps[$i]} | awk '{print $1}'`
		if [ $size != ${snapsize[$i]} ] && [ $size != `expr ${snapsize[$i]} + 1` ]; then
			echo "Test Failed";
			echo "Snap: $i Name: ${snaps[$i]} Actual: $size Expected: ${snapsize[$i]}"
			collect_logs_and_exit
		fi
		size=`du -sh --block-size=1048576 /tmp/vol2/${snaps[$i]} | awk '{print $1}'`
		if [ $size != ${snapsize[$i]} ] && [ $size != `expr ${snapsize[$i]} + 1` ]; then
			echo "Test Failed";
			echo "Snap: $i Name: ${snaps[$i]} Actual: $size Expected: ${snapsize[$i]}"
			collect_logs_and_exit
		fi
	done
}

update_file_sizes() {
	active_file_size=$1
	i=0
	for x in "$@"
	do
		if [[ $i == 0 ]]; then
			let i=i+1
			continue
		fi
		snapsize[$i]=$x
		let i=i+1
	done
}

test_restart_during_prepare_rebuild() {
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	verify_replica_cnt "2" "test restart during add"

	login_to_volume "$CONTROLLER_IP:3260"
	sleep 5
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		mkfs.ext2 -F /dev/$device_name
		mount /dev/$device_name /mnt/store
		if [ $? -ne 0 ]; then
			echo "mount failed in test_restart_during_add_replica"
			collect_logs_and_exit
		fi
		umount /mnt/store
	else
		echo "Unable to detect iSCSI device during test_restart_add_replica"; collect_logs_and_exit
	fi
	logout_of_volume

	docker stop $replica2_id
	sleep 1
	verify_replica_mode 1 "replica mode in test with restart during prepare rebuild" "$REPLICA_IP1" "RW"
	replica2_id=$(start_debug_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2" "PANIC_AFTER_PREPARE_REBUILD" "TRUE")
	verify_container_dead $replica2_id "restart during prepare rebuild"
	docker stop $replica2_id
        replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	rev_counter1=`cat /tmp/vol1/revision.counter`
	rev_counter2=`cat /tmp/vol2/revision.counter`
	if [ $rev_counter1 -ne $rev_counter2 ]; then
		echo "replica restart during prepare rebuild1 -- failed"
		collect_logs_and_exit
	fi
	if [ $rev_counter1 -lt 5 ]; then
		echo "replica restart during prepare rebuild2 -- failed"
		collect_logs_and_exit
	fi
	cleanup
}

test_duplicate_data_delete() {
	echo "----------------Test_duplicate_data_delete---------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "2")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	verify_replica_cnt "2" "Duplicate data delete test"
	snaps[$snapIndx]=`ls /tmp/vol1 | grep .img | grep -v meta | grep  -v head`
	let snapIndx=snapIndx+1
	active_file1=`ls /tmp/vol1 | grep .img | grep -v meta | grep head`
	active_file2=`ls /tmp/vol2 | grep .img | grep -v meta | grep head`
	active_file_size=0
	update_file_sizes 0 0
	verify_physical_space_consumed
	# Size: 0M, Offsets Filled: [) in Active file
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	run_ios 5K 0
	update_file_sizes 20 0
	verify_physical_space_consumed
	# Size: 20M, Offsets Filled: [0,5K) in Active file
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	create_auto_generated_snapshot "snap2"
	update_file_sizes 0 0 20
	verify_physical_space_consumed
	# Size: 0M, Offsets Filled: [) in Active file
	# Size: 20M, Offsets Filled: [0,5K) in Snap2(Auto generated)
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	run_ios 5K 0
	update_file_sizes 20 0 0
	verify_physical_space_consumed
	# Size: 20M, Offsets Filled: [0,5K) in Active File
	# Size: 0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	create_auto_generated_snapshot "snap3"
	update_file_sizes 0 0 0 20
	verify_physical_space_consumed
	# Size: 0M, Offsets Filled: [) in Active file
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size: 0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	run_ios 5K 5K
	update_file_sizes 20 0 0 20
	verify_physical_space_consumed
	# Size: 20M, Offsets Filled: [5K,10K) in Active file
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size: 0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	create_manual_snapshot "snap4"
	update_file_sizes 0 0 0 20 20
	verify_physical_space_consumed
	base_size=`du -sh --block-size=1048576 /tmp/vol1/| awk '{print $1}'`
	# Size: 0M, Offsets Filled: [) in Active file
	# Size: 20M, Offsets Filled: [5K,10K) in Snap4(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size: 0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	run_ios 10K 0
	update_file_sizes 40 0 0 20 20
	verify_physical_space_consumed
	# Size: 40M, Offsets Filled: [0,10K) in Active file
	# Size: 20M, Offsets Filled: [5K,10K) in Snap4(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size: 0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	create_auto_generated_snapshot "snap5"
	update_file_sizes 0 0 0 20 20 40
	verify_physical_space_consumed
	# Size: 0M, Offsets Filled: [) in Active file
	# Size: 40M, Offsets Filled: [0,10K) in Snap5(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap4(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size: 0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	run_ios 5K 5K
	update_file_sizes 20 0 0 20 20 20
	verify_physical_space_consumed
	# Size: 20M, Offsets Filled: [5K,10K) in Active file
	# Size: 20M, Offsets Filled: [0,5K) in Snap5(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap4(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size: 0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	create_manual_snapshot "snap6"
	update_file_sizes 0 0 0 20 20 20 20
	verify_physical_space_consumed
	# Size: 0M, Offsets Filled: [) in Active File
	# Size: 20M, Offsets Filled: [5K,10K) in Snap6(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap5(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap4(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size: 0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	run_ios 10K 0
	update_file_sizes 40 0 0 20 20 20 20
	verify_physical_space_consumed
	# Size: 40M, Offsets Filled: [0,10K) in Active File
	# Size: 20M, Offsets Filled: [5K,10K) in Snap6(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap5(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap4(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size: 0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size: 0M, Offsets Filled: [) in Snap1(Auto generated)

	create_auto_generated_snapshot "snap7"
	update_file_sizes 0 0 0 20 20 20 20 40
	verify_physical_space_consumed
	# Size:  0M, Offsets Filled: [) in Active File
	# Size: 40M, Offsets Filled: [0, 10K) in Snap7(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap6(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap5(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap4(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size:  0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size:  0M, Offsets Filled: [) in Snap1(Auto generated)

	run_ios 4K 4K
	update_file_sizes 16 0 0 20 20 20 20 24
	verify_physical_space_consumed
	# Size: 16M, Offsets Filled: [4k,8K) in Active File
	# Size:  24M, Offsets Filled: [0,4K)[8K, 10K) in Snap7(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap6(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap5(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap4(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size:  0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size:  0M, Offsets Filled: [) in Snap1(Auto generated)

	# Test non-contiguous blocks deletion in the same file
	run_ios 6k 3K
	update_file_sizes 24 0 0 20 20 20 20 16
	verify_physical_space_consumed
	# Size: 24M, Offsets Filled: [3k,9K) in Active File
	# Size: 16M, Offsets Filled: [0,3K)[9K, 10K) in Snap7(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap6(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap5(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap4(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size:  0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size:  0M, Offsets Filled: [) in Snap1(Auto generated)


	##Verify API to update disk mode
	create_auto_generated_snapshot "snap8"
	update_file_sizes 0 0 0 20 20 20 20 16 24
	verify_physical_space_consumed
	# Size:  0M, Offsets Filled: [) in Active File
	# Size: 24M, Offsets Filled: [3k,9K) in Snap8(Auto Generated)
	# Size: 16M, Offsets Filled: [0,3k)[9K, 10K) in Snap7(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap6(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap5(Auto Generated)
	# Size: 20M, Offsets Filled: [5K,10K) in Snap4(User Created)
	# Size: 20M, Offsets Filled: [0,5K) in Snap3(Auto Generated)
	# Size:  0M, Offsets Filled: [) in Snap2(Auto generated)
	# Size:  0M, Offsets Filled: [) in Snap1(Auto generated)

	echo "Test duplicate data delete passed"
	docker stop $replica1_id
	docker stop $replica2_id
	docker stop $orig_controller_id
	cleanup
}

# run_dd run io's on the block device in $1 file and skip $2 no of
# blocks in $1 file.
run_dd() {
	dd if=/dev/urandom of=$1 bs=4k count=100000 seek=$2
        sync
        hash1=""
        hash1=$(md5sum $1 | awk '{print $1}')
        echo "$hash1"
}


# copy_files_into_mnt_dir login to iscsi target and discovers block device
# format it and copy's $1 and $2 files
copy_files_into_mnt_dir() {
	echo "------------------Copy files into mnt dir------------------"
	login_to_volume "$CONTROLLER_IP:3260"
	sleep 5
	get_scsi_disk
	if [ "$device_name"!="" ]; then
		mkfs.ext2 -F /dev/$device_name

		mount /dev/$device_name /mnt/store

		cp $1 /mnt/store
                if [ "$2" != "" ]; then
                        cp $2 /mnt/store
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

verify_delete_snapshot_failure() {
	echo "-------------Verify delete snapshots failure---------------"
        declare -a errors=("Failed to coalesce" "Replica tcp://"$1":9502 mode is" "Snapshot deletion process is in progress" "Can not remove a snapshot because, RF: 3, replica count: 2" "Client.Timeout exceeded while awaiting headers")
        local i=""
        local cnt=0
        local snap=""
        local msg=""
        local snap_ls="docker exec "$orig_controller_id" jivactl snapshot ls"
        snap=$(eval $snap_ls | tail -1)
        cmd="docker exec "$orig_controller_id" jivactl snapshot rm "$snap""
        msg=$(eval $cmd 2>&1)
        echo "$msg"
        for i in "${errors[@]}"
        do
                # match the substring with the given array of strings
                if [[ "$msg" == *"$i"* ]]; then
                        cnt=$((cnt + 1))
                        echo "Verify delete snapshot failure -- passed"
                        return
                fi
        done
        if [ "$cnt" == "0" ] && [ "$2" == "true" ]; then
                if [[ "$msg" == *"deleted snapshot"* ]]; then
                        echo "Verify delete snapshot failure -- passed"
                        return
                else
                        echo "Verify delete snapshot failure -- failed"
                        collect_logs_and_exit
                        return
                fi
        elif [ "$cnt" == "0" ]; then
                echo "Verify delete snapshot failure -- failed"
                collect_logs_and_exit
        fi

}

# delete_snapshot delete all the snapshot except for Head and latest snapshot.
delete_snapshots() {
	echo "---------------------delete snapshots----------------------"
        local snaps=""
        local i=3
        local lines=""
        snaps=$(docker exec -it "$orig_controller_id" jivactl snapshot ls)
        lines=$(echo "$snaps" | wc -w)
        while [ "$i" -lt "$lines" ]
        do
                snaps=$(docker exec "$orig_controller_id" jivactl snapshot ls)
                snap=$(echo $snaps | awk '{print $3}')
                docker exec -it "$orig_controller_id" jivactl snapshot rm $snap
                i=$((i + 1))
        done
}

# mount_and_verify_hash does login to iscsi target and mount the block
# device to /mnt/store dir and verifies checksum ($1, $3) of file $2
# and $4 respectively.
mount_and_verify_hash() {
	echo "--------------------Mount and verify hash------------------"
       	login_to_volume "$CONTROLLER_IP:3260"
	sleep 5
	get_scsi_disk
	if [ "$device_name" != "" ]; then
		mount /dev/$device_name /mnt/store
		hash1=$(md5sum /mnt/store/$2 | awk '{print $1}')
                if [ "$3" != "" ] && [ "$4" != "" ]; then
                        hash2=$(md5sum /mnt/store/$4 | awk '{print $1}')
                        if [ "$1" == "$hash1" ] && [ "$3" == "$hash2" ]; then echo "DI Test: PASSED"
                        else
                                echo "Mount and verify hash Test: FAILED"; collect_logs_and_exit
                        fi
                else
                        if [ "$1" == "$hash1" ]; then echo "DI Test: PASSED"
                        else
                                echo "Mount and verify hash Test: FAILED"; collect_logs_and_exit
                        fi
                fi

		umount /mnt/store
		logout_of_volume
		sleep 5
	else
		echo "Unable to detect iSCSI device, login failed"; collect_logs_and_exit
	fi
}

# start_stop_replica start and stop replica $2 for $1 no of times
start_stop_replica() {
	echo "----------------------Start stop replica---------------------"
        i=0
        while [ "$i" -lt "$1" ];
        do
                sleep 2
                sudo docker stop $2
                sudo docker start $2
                i=$((i + 1))
        done
}

# verify if snapshot deletion has been failed while replica restarts
# and other cases. $1 and $2 are the files that to be
verify_replica_restart_while_snap_deletion() {
	echo "--------Verify replica restart while snap deletion--------"
        copy_files_into_mnt_dir "$1" "$2" &
        sleep 5
        start_stop_replica "4" "$3"
        verify_replica_cnt "3" "Three replica count test"
        sudo docker stop "$3"
        wait
        # case 1: Rebuild in progress
        sudo docker start "$3"
        sleep 5
        verify_delete_snapshot_failure "$4" "false"
        verify_rw_rep_count "3"

        # case 2: Replication factor is not met
        sudo docker stop "$3"
        sleep 5
        verify_delete_snapshot_failure "$4" "false"

        # case 3: Coalesce failed or client timeout exceeded
        sleep 2
        sudo docker start "$3"
        verify_replica_cnt "3" "Three replica count test"
        verify_rw_rep_count "3"
        verify_delete_snapshot_failure "$4" & "false"
        sleep 2
        sudo docker stop "$3"
        wait

        # case 4: Already a snapshot being deleted
        sudo docker start "$3"
        verify_replica_cnt "3" "Three replica count test"
        verify_rw_rep_count "3"
        verify_delete_snapshot_failure "$4" "true" &
        sleep 0.5
        verify_delete_snapshot_failure "$4" "false"
        wait
}

test_delete_snapshot() {
	echo "--------------------Test_delete_snapshot------------------"
	orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
	replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
	replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
	replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")

        verify_replica_cnt "3" "Three replica count test"
        device_name=""
        file1="f1"
        file2="f2"
        file3="f3"
        hash_file1=""
        hash_file2=""
        hash_file3=""
        hash_file1=$(run_dd "$file1" "0")
        hash_file2=$(run_dd "$file2" "100000")
        copy_files_into_mnt_dir "$file1" "$file2" &

        # start stop replica such that data is written across various
        # snapshots
        sleep 1
        start_stop_replica "4" "$replica3_id"
        wait

        verify_replica_cnt "3" "Three replica count test"
        verify_rw_rep_count "3"

        delete_snapshots

        mount_and_verify_hash "$hash_file1" "$file1" "$hash_file2" "$file2"

        # overwriting on the same blocks and verify data consistency
        hash_file3=$(run_dd "$file3" "0")
        copy_files_into_mnt_dir "$file3" &

        start_stop_replica "2" "$replica3_id"
        wait

        verify_replica_cnt "3" "Three replica count test"
        verify_rw_rep_count "3"

        delete_snapshots
        mount_and_verify_hash "$hash_file3" "$file3"
        cleanup
}

test_replica_restart_while_snap_deletion() {
        echo "------------Test replica restart while snap deletion---------------"
        orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
        replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
        replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
        replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")

        verify_replica_cnt "3" "Three replica count test"
        verify_replica_restart_while_snap_deletion "f1" "f2" "$replica3_id" "$REPLICA_IP3"
        rm -vrf "f1" "f2" "f3"
        cleanup
}

test_replica_restart_optimization() {
        echo "------------Test replica restart optimization---------------"
        orig_controller_id=$(start_controller "$CONTROLLER_IP" "store1" "3")
        replica1_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP1" "vol1")
        replica2_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP2" "vol2")
        replica3_id=$(start_replica "$CONTROLLER_IP" "$REPLICA_IP3" "vol3")

        run_ios 100K 0 &
        sleep 10
        docker stop $replica3_id
        wait
        docker stop $replica2_id
        docker stop $replica1_id
        sleep 5
        docker start $replica3_id
        docker start $replica2_id
        sleep 3
        docker start $replica1_id
        verify_replica_cnt "3" "Thre replica count test"
        count=$(docker logs $orig_controller_id 2>&1 | grep -c "Replica tcp://172.18.0.3:9502 will takeover")
        if [ "$count" -eq 0  ]; then
           echo "replica restart optimization test failed"
           collect_logs_and_exit
        fi
           echo "replica restart optimization test --passed"

        cleanup
}

prepare_test_env
test_replica_restart_optimization
test_delete_snapshot
test_replica_restart_while_snap_deletion
test_duplicate_data_delete
test_single_replica_stop_start
test_restart_during_prepare_rebuild
test_two_replica_stop_start
test_three_replica_stop_start
test_ctrl_stop_start
test_replica_controller_continuous_stop_start
test_restart_during_prepare_rebuild
#test_bad_file_descriptor
test_preload
test_replica_rpc_close
test_controller_rpc_close
test_replication_factor
#test_two_replica_delete
test_replica_ip_change
test_replica_reregistration
test_volume_resize
run_data_integrity_test_with_fs_creation
test_clone_feature
test_duplicate_snapshot_failure
#test_extent_support_file_system
test_upgrades
run_vdbench_test_on_volume
run_libiscsi_test_suite
