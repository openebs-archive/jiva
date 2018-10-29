#!/bin/bash

. ci/libs/variables.sh

cleanup(){
	rm -rf $TMP_DIR/vol*
	rm -rf $MNT_DIR/logs
	docker stop $(docker ps -aq)
	docker rm $(docker ps -aq) 
}

# start_controller CONTROLLER_IP
start_controller() {
	controller_id=$(docker run -d \
	    --net stg-net \
	    --ip $1\
	    -P --expose 3260 --expose 9501 --expose 9502-9504 \
	    $JI \
	    env REPLICATION_FACTOR="$3" \
	    launch controller \
	    --frontend gotgt\
	    --frontendIP "$1" "$2")
	echo "$controller_id"
}

# start_replica CONTROLLER_IP REPLICA_IP folder_name
start_replica() {
	replica_id=$(docker run -d -it \
	    --net stg-net --ip "$2"\
	    -P --expose 9502-9504 \
	    -v $TMP_DIR/"$3":/"$3" \
	    $JI \
	    launch replica \
	    --frontendIP "$1" \
	    --listen "$2":9502\
	    --size 2g /"$3")
	echo "$replica_id"
}

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

	echo "ls VOL1>>"
	ls -ltr $TMP_DIR/vol1/
	echo "ls VOL2>>"
	ls -ltr $TMP_DIR/vol2/
	echo "ls VOL3>>"
	ls -ltr $TMP_DIR/vol3/
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

verify_replica_cnt() {
	i=0
	replica_cnt=""
	while [ "$replica_cnt" != "$1" ]; do
		date
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
			echo "verify_rw_status -- failed"
			collect_logs_and_exit
		fi
		sleep 2
	done
	echo "0"
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
