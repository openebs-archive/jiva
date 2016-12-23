#!/bin/bash

sudo docker network create --subnet=172.18.0.0/16 longhorn-net
sudo docker run -d -it --net longhorn-net --ip 172.18.0.3 --expose 9502-9504 -v /mnt/vol1:/vol1  $1 launch replica --frontendIP 172.18.0.2 --listen 172.18.0.3:9502 --size 10g /vol1
sudo docker run -d --net longhorn-net --ip 172.18.0.2 -P --expose 3260 --expose 9501  $1 launch controller --frontend gotgt --frontendIP 172.18.0.2 --replica tcp://172.18.0.3:9502 store1
sudo docker run -d -it --net longhorn-net --ip 172.18.0.4 --expose 9502-9504 -v /mnt/vol2:/vol2 $1 launch replica --frontendIP 172.18.0.2 --listen 172.18.0.4:9502 --size 10g /vol1

sudo umount /mnt/store
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
