# jiva

[![Build Status](https://travis-ci.org/openebs/jiva.svg?branch=master)](https://travis-ci.org/openebs/jiva)

Jiva is a Docker image for creating OpenEBS VSMs / Storage Pods. OpenEBS VSM/Storage Pod allows you to create persistent or ephimeral storage for your containers by pooling local storage from the hard disks connected to your machines. To expose the storage you will need to launch the following containers:

(a) Create a frontend container that exposes storage using tcmu or iscsi. 

```
docker run -d --network="host" -P --expose 3260 --expose 9502-9504 --expose 9501 openebs/jiva launch controller --frontend gotgt --frontendIP <frontendIP>
```

(b) Add one more storage/backend containers that will persist the data. The frontend container will replicate the data to all the backend containers added to it. 

```
docker run -d -it --network="host" -P --expose 9502-9504 -v /mnt/stor1:/stor1 <openebs/jiva> launch replica --frontendIP <frontendIP> --listen <ReplicaIp>:9502 --size <size> /stor1
```

The frontend container also provides an API to manage storage like adding/removing replicas, managing snapshots etc., 

For further info, checkout http://www.openebs.io
