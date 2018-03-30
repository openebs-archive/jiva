#!/bin/bash

# Get Docker image for Jiva
JI=$(sudo docker images | grep openebs/jiva | awk '{print $1":"$2}')
echo "Run CI tests on $JI"

# Prepare environment to run Jiva containers
sudo rm -rf /tmp/vol*
sudo rm -rf /mnt/logs
mkdir /tmp/vol1
mkdir /tmp/vol2
mkdir /tmp/vol3
sudo docker network create --subnet=172.18.0.0/16 stg-net

# Start Jiva controller and replicas in detached mode
sudo docker run -d --net stg-net --ip 172.18.0.2 -P --expose 3260 --expose 9501 --expose 9502-9504 $JI launch controller --frontend gotgt --frontendIP 172.18.0.2 store1
sudo docker run -d -it --net stg-net --ip 172.18.0.3 -P --expose 9502-9504 -v /tmp/vol1:/vol1 $JI launch replica --frontendIP 172.18.0.2 --listen 172.18.0.3:9502 --size 2g /vol1
sudo docker run -d -it --net stg-net --ip 172.18.0.4 -P --expose 9502-9504 -v /tmp/vol2:/vol2 $JI launch replica --frontendIP 172.18.0.2 --listen 172.18.0.4:9502 --size 2g /vol2

# Display running containers
sudo docker ps

# Create a local mountpoint
sudo mkdir -p /mnt/store
sudo mkdir -p /mnt/store2

# Cleanup existing iSCSI sessions
logout_of_volume() {
	sudo iscsiadm -m node -u
	sudo iscsiadm -m node -o delete
}

# Discover Jiva iSCSI target and Login
login_to_volume() {
	sudo iscsiadm -m discovery -t st -p $1
	sudo iscsiadm -m node -l
}

# Wait for iSCSI device node (scsi device) to be created
get_scsi_disk() {
	device_name=$(iscsiadm -m session -P 3 |grep -i "Attached scsi disk" | awk '{print $4}')
	i=0
	while [ -z $device_name ]; do
		sleep 4
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

login_to_volume "172.18.0.2:3260"
sleep 5
get_scsi_disk
if [ "$device_name"!="" ]; then
	# Format disk as ext2 FS
	sudo mkfs.ext2 -F /dev/$device_name

	# Mount FS onto local mountpoint
	sudo mount /dev/$device_name /mnt/store

	# TEST#1: Perform simple data-integrity check on Jiva Vol
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

#Create a snapshot for testing clone feature
id=`curl http://172.18.0.2:9501/v1/volumes | jq '.data[0].id' |  tr -d '"'`
curl -H "Content-Type: application/json" -X POST -d '{"name":"snap1"}' http://172.18.0.2:9501/v1/volumes/$id?action=snapshot

# TEST#2: Perform a random I/O workload test on Jiva Vol
login_to_volume "172.18.0.2:3260"
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

# TEST#3: Run the libiscsi compliance suite on Jiva Vol
echo "Run the libiscsi compliance suite on Jiva Vol"
sudo mkdir /mnt/logs
sudo docker run -v /mnt/logs:/mnt/logs --net host openebs/tests-libiscsi /bin/bash -c "./testiscsi.sh --ctrl-svc-ip 172.18.0.2"
tp=$(grep "PASSED" $(find /mnt/logs -name SUMMARY.log) | wc -l)
tf=$(grep "FAILED" $(find /mnt/logs -name SUMMARY.log) | wc -l)
if [ $tp -ge 146 ] && [ $tf -le 29 ]; then
	echo "iSCSI Compliance test: PASSED"
else
	echo "iSCSI Compliance test: FAILED"; exit 1
fi

# TEST#4: Test clone feature
echo "Test clone feature"
sudo docker run -d --net stg-net --ip 172.18.0.5 -P --expose 3260 --expose 9501 --expose 9502-9504 $JI launch controller --frontend gotgt --frontendIP 172.18.0.5 store1
sudo docker run -d -it --net stg-net --ip 172.18.0.6 -P --expose 9502-9504 -v /tmp/vol3:/vol3 $JI launch replica --type clone --snapName snap1 --cloneIP 172.18.0.2 --frontendIP 172.18.0.5 --listen 172.18.0.6:9502 --size 2g /vol3

clonestatus=`curl http://172.18.0.6:9502/v1/replicas/1 | jq '.clonestatus' | tr -d '"'`

i=0
while [ -z $clonestatus ]; do
	sleep 3
	clonestatus=`curl http://172.18.0.6:9502/v1/replicas/1 | jq '.clonestatus' | tr -d '"'`
	i=`expr $i + 1`
	if [ $i -eq 20 ]; then
		echo "Clone process took longer than usual"
		exit 1;
	else
		echo "Waiting for clone process to complete"
		continue
	fi
done

# Display running containers
sudo docker ps

# Display volume info
curl http://172.18.0.5:9501/v1/volumes

# Discover Jiva iSCSI target and Login
sleep 5
login_to_volume "172.18.0.5:3260"
get_scsi_disk

if [ "$device_name"!="" ]; then
	# Mount FS onto local mountpoint
	sudo mount /dev/$device_name /mnt/store2

	# TEST#1: Perform simple data-integrity check on Jiva Vol
	hash3=$(sudo md5sum /mnt/store2/file1 | awk '{print $1}')
	if [ $hash1 == $hash3 ]; then
		sudo umount /mnt/store2
		logout_of_volume
		echo "DI Test: PASSED"
	else
		sudo umount /mnt/store2
		logout_of_volume
		echo "DI Test: FAILED"; exit 1
	fi
else
	echo "Unable to detect iSCSI device, login failed"; exit 1
fi
