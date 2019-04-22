# jiva

[![Build Status](https://travis-ci.org/openebs/jiva.svg?branch=master)](https://travis-ci.org/openebs/jiva)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fopenebs%2Fjiva.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fopenebs%2Fjiva?ref=badge_shield)

Jiva is a Docker image for creating OpenEBS Volumes. OpenEBS Volumes provide block storage to your application containers. An OpenEBS Volume - is itself launched as a container, making OpenEBS a true Container Native Storage for Containers.

An OpenEBS Volume is comprised of:
- One *controller* (or Frontend-Container) that exposes storage via iSCSI, TCMU etc., and can be launched alongside your application container.
- One or more *replicas* (or Backend-Containers) that will persist the data, providing redundancy and high-availability. The replica container will save the data onto the storage (disks) attached to your docker hosts.

![Jiva Overview Diagram](https://github.com/openebs/openebs/blob/master/documentation/source/_static/JivaExample.png)

Both controller and replica can be launched using this same image, *openebs/jiva*, by passing different launch arguments. The controller expects volume details like name and size. The replica will require the controller to which it is attached and the mount path where it stores the data.

The scheduling of the controller and replica containers of an OpenEBS Volume is managed by *maya* that integrates itself into Container Orchestrators like Kubernetes, Docker Swarm, Mesos or Nomad.

OpenEBS v0.4.0, provides integration into Kubernetes Clusters. Please follow [OpenEBS documentation](https://docs.openebs.io/), if you are using Kubernetes.

For other types of Container Orchestrators like Docker Swarm, Mesos, Nomad, etc., the following instructions will help you to quickly setup an OpenEBS Volume (exposed via iSCSI) with 2 replicas. To enable redundancy, the replicas have to be run on different hosts.

## Building from source code

### Prerequisites:

Requires *curl* and *docker* to be installed on your build machine. 

### Build

`make build`

- Downloads *dapper* using curl.
- Downloads *trash* using go get for dependency management.
- Triggers a build using *dapper* - wrapper around docker.


## Running in standalone

### Prerequisites

#### Setup the storage network.

To launch a OpenEBS Volume with 2 replicas, 3 IPs will be required. 1 for the controller (where the iSCSI service will be started) and 2 for the replicas. This example uses static IPs with host networking mode which can be setup using your preferred network plugin that allows multi-host networking.

For Example:

Let us say that 172.18.200.0/24 is accessible across different docker hosts (host#1, host#2).

Create the static IPs on host#1, where you can create the controller and replica-1.

```
host#1$ sudo ip addr add 172.18.200.101/24 dev eth0
host#1$ sudo ip addr add 172.18.200.102/24 dev eth0
```

Create the static IP on host#2, where you can create replica-2.

```
host#2$ sudo ip addr add 172.18.200.103/24 dev eth0
```

#### Setup the storage disks

The backend containers will need to be provided with the storage (directory) where the data will be persisted. The directory should be accessible via the containers. One host1 and host2 setup creates a directory on the disk that has sufficient space for holding the volume. (Again, these operations are taken care by the Storage Orchestrator - Maya, if you are using Kubernetes).

```
host#1$ mkdir /mnt/store1
```
```
host#2$ mkdir /mnt/store2
```

### Launch controller

Create a controller on host#1, that will create *demo-vol1* with capacity of 10G.

```
host#1$ sudo docker run -d --net host -P --expose 3260 --expose 9501 --name ctrl openebs/jiva launch controller --frontend gotgt --frontendIP 172.18.200.101 demo-vol1 10G
```

### Launch replica

Create the replica on host#1, for demo-vol1 with data stored in /mnt/store1.

```
docker run -d --network="host" -P --expose 9502-9504 --expose 9700-9800 -v /mnt/stor1:/stor1 --name replica-1 <openebs/jiva> launch replica --frontendIP 172.18.200.101 --listen 172.18.200.102:9502 --size 10G /stor1
```

Create the backend container on host#2, for demo-vol1 with data stored in /mnt/store1.

```
docker run -d --network="host" -P --expose 9502-9504 --expose 9700-9800 -v /mnt/stor2:/stor2 --name replica-2 <openebs/jiva> launch replica --frontendIP 172.18.200.101 --listen 172.18.200.103:9502 --size 10G /stor2
```

### Using the iSCSI volume

The volume created from the above containers can be used via the iSCSI plugin or iscsiadm by using the following details:

Portal : 172.18.200.101:3260
IQN : iqn.2016-09.com.openebs.jiva:demo-vol1


*For further info, checkout https://docs.openebs.io/ or join the slack# https://slack.openebs.io*

## Inspiration and Credit

[Rancher Longhorn](https://github.com/rancher/longhorn), which helped with the significant portion of the code in this repo, helped us to validate that Storage controllers can be run as microservices and they can be coded in Go. The iSCSI functionality is derived from [gostor/gotgt](https://github.com/gostor/gotgt).

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fopenebs%2Fjiva.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fopenebs%2Fjiva?ref=badge_large)
