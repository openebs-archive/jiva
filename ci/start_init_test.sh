#!/bin/bash

# Show commands executed
# set -x 

#imports
. ci/libs/variables.sh
. ci/libs/utils.sh

prepare_test_env() {
	echo "-------------------Prepare test env------------------------"
	cleanup
    sleep 5

	mkdir -p $TMP_DIR/vol1 $TMP_DIR/vol2 $TMP_DIR/vol3 $TMP_DIR/vol4
	mkdir -p $MNT_DIR/store $MNT_DIR/store2
    network=$(docker network inspect stg-net | grep Error)
    if [ -z network ]; then
        docker network create --subnet=172.18.0.0/16 stg-net
    fi
	export JI=$(docker images | grep openebs/jiva | awk '{print $1":"$2}' | awk 'NR == 2 {print}')
	export JI_DEBUG=$(docker images | grep openebs/jiva | awk '{print $1":"$2}' | awk 'NR == 1 {print}')
	echo "Run CI tests on $JI and $JI_DEBUG"
}

prepare_test_env

for test in ci/tests/*.sh; do
    $test
done
