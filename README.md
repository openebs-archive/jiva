# jiva

[![Build Status](https://travis-ci.org/openebs/jiva.svg?branch=master)](https://travis-ci.org/openebs/jiva)

Jiva is a Docker image for creating OpenEBS VSMs / Storage Pods. 

OpenEBS VSM/Storage Pod allows you to create persistent or ephemeral storage for your containers by pooling local storage from the hard disks connected to your machines. 

An OpenEBS Storage comprises of an Frontend-Container that exposes storage via iSCSI, TCMU etc., and can be launched along side your application container. Each Frontend Container can be then associated with one or more Backend-Containers that will persist the data, providing redundancy and high-availability. 

![Jiva Overview Diagram](https://github.com/openebs/openebs/blob/master/documentation/source/_static/JivaExample.png)

The following instructions will help you to quickly setup an OpenEBS iSCSI volume with 2 backend containers that can be used for your Persistent Storage needs. To enable redundancy, the backend containers have to be run on different hosts. 

### Prerequisites

#### Setup the storage network. 

To launch a storage with 2 backend containers, you will require three IPs one for the frontend container and 2 for the backend container. For redundancy, the backend containers should be launched on different docker hosts. This example uses static IPs with host networking mode, but you can set this up using any of your preferred network plugin, that allows multi-host networking.

Let us say that 172.18.200.0/24 is accessible across different hosts ( host#1, host#2). 

Create the static IPs for host#1, Frontend container and Backend Container-1.

```
host#1$ sudo ip addr add 172.18.200.101/24 dev eth0
host#1$ sudo ip addr add 172.18.200.102/24 dev eth0
```

Create the static IP for host#2 for BackendContainer-2. 

```
host#2$ sudo ip addr add 172.18.200.103/24 dev eth0
```

#### Setup the storage disks

The backend containers will need to provided with the storage (directory) where the data will be persisted. The directory should be accessible via the containers.  One host1 and host2, setup create a directory on the disk that has sufficient space for holding the volume. 

```
host#1$ mkdir /mnt/store1
```

```
host#2$ mkdir /mnt/store2
```


### Launch Frontend Container

Create a frontend container on host#1, that will create *demo-vol1* with capacity of 10G. 

```
host#1$ sudo docker run -d --network="host" -P --expose 3260 --expose 9501 --name sp-fe openebs/jiva launch controller --frontend gotgt --frontendIP 172.18.200.101 demo-vol1 10G
```
  

### Launch Backend Containers

Create the backend container on host#1, for demo-vol1 with data stored in /mnt/store1

```
docker run -d --network="host" -P --expose 9502-9504 --expose 9700-9800 -v /mnt/stor1:/stor1 --name sp-be1 <openebs/jiva> launch replica --frontendIP 172.18.200.101 --listen 172.18.200.102:9502 --size 10G /stor1
```
    
Create the backend container on host#2, for demo-vol1 with data stored in /mnt/store1

```
docker run -d --network="host" -P --expose 9502-9504 --expose 9700-9800 -v /mnt/stor2:/stor2 --name sp-be2 <openebs/jiva> launch replica --frontendIP 172.18.200.101 --listen 172.18.200.103:9502 --size 10G /stor2
```

### Using the iSCSI volume

The volume created from the above containers can be used via the iSCSI plugin or iscsiadm by using the following details:

Portal : 172.18.200.101:3260
IQN : iqn.2016-09.com.openebs.jiva:demo-vol1




For further info, checkout http://www.openebs.io
