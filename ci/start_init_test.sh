#!/bin/bash

JI=$(sudo docker images | grep jiva | awk '{print $1":"$2}')
echo "Run CI tests on $JI"

mkdir /tmp/vol1
mkdir /tmp/vol2
sudo docker network create --subnet=172.18.0.0/16 stg-net

sudo docker run -d --net stg-net --ip 172.18.0.2 -P --expose 3260 --expose 9501 --expose 9502-9504 $JI launch controller --frontend gotgt --frontendIP 172.18.0.2 store1
sudo docker run -d -it --net stg-net --ip 172.18.0.3 -P --expose 9502-9504 -v /tmp/vol1:/vol1 $JI launch replica --frontendIP 172.18.0.2 --listen 172.18.0.3:9502 --size 2g /vol1
sudo docker run -d -it --net stg-net --ip 172.18.0.4 -P --expose 9502-9504 -v /tmp/vol2:/vol2 $JI launch replica --frontendIP 172.18.0.2 --listen 172.18.0.4:9502 --size 2g /vol2

sudo docker ps

sudo mkdir -p /mnt/store
sudo iscsiadm -m node -u
sudo iscsiadm -m node -o delete
sudo iscsiadm -m discovery -t st -p 172.18.0.2:3260
sudo iscsiadm -m node -l
sleep 1

sudo fdisk -l
x=$(iscsiadm -m session -P 3 |grep -i "Attached scsi disk" | awk '{print $4}')
echo $x

i=0
while x==""; do
        sleep 4
        x=$(iscsiadm -m session -P 3 |grep -i "Attached scsi disk" | awk '{print $4}')
        i=i+1
        if i==5; then
                break
        else
                continue
        fi
done

if [ "$x"!="" ]; then
        sudo mkfs.ext2 -F /dev/$x
        sudo mount /dev/$x /mnt/store
        sudo dd if=/dev/urandom of=/mnt/store/file1 bs=4k count=10000
        sudo md5sum /mnt/store/file1 > file1
fi

sudo mkdir -p /mnt/store/data
sudo chown 777 /mnt/store/data
sudo docker run -v /mnt/store/data:/datadir1 openebs/tests-vdbench:latest
