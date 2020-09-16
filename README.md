# Jiva

[![Build Status](https://travis-ci.org/openebs/jiva.svg?branch=master)](https://travis-ci.org/openebs/jiva)
[![Go Report Card](https://goreportcard.com/badge/github.com/openebs/jiva)](https://goreportcard.com/report/github.com/openebs/jiva)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/616f61627a4543febe14af30358805b9)](https://www.codacy.com/app/OpenEBS/jiva?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=openebs/jiva&amp;utm_campaign=Badge_Grade)
[![GoDoc](https://godoc.org/github.com/openebs/jiva?status.svg)](https://godoc.org/github.com/openebs/jiva)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fopenebs%2Fjiva.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fopenebs%2Fjiva?ref=badge_shield)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1755/badge)](https://bestpractices.coreinfrastructure.org/projects/1755)
[![Community Meetings](https://img.shields.io/badge/Community-Meetings-blue)](https://hackmd.io/hiRcXyDTRVO2_Zs9fp0CAg)

Jiva provides highly available iSCSI block storage Persistent Volumes for Kubernetes Stateful Applications, by making use of the host filesystem.

Jiva comprises of two components:
-   A Target ( or a Storage Controller) that exposes iSCSI, while synchronously replicating the data to one or more Replicas. 
-   A set of Replicas that a Target uses to read/write data. Each of the replicas will be on a different node to ensure high availability against node or network failures. The Replicas save the data into sparse files on the host filesystem directories. 

Jiva is containerized storage controller. The docker images are available at:
-   [Docker Hub](https://cloud.docker.com/u/openebs/repository/docker/openebs/jiva)
-   [Quay](https://quay.io/repository/openebs/jiva)

The docker container can be used directly to spin up the volume. OpenEBS Control Plane makes it easy to manage Jiva Volumes. 

When using Jiva volumes with OpenEBS, the Jiva volumes is composed of the following Kubernetes native objects.
-   A Kubernetes Service pointing to Jiva iSCSI Target
-   A Kubernetes Deployment for Jiva Target with Replicas=1
-   A Kubernetes Deployment for Jiva Replicas with Replica=n and Pod Anti-affinity set to have each Replica pod on different node

The Jiva Replica Pods are provided with Jiva iSCSI Target Service - Cluster IP  and can auto-connect/register with the Targets. The Jiva Target helps in determining the consistency of data across the replicas, by taking care of rebuilding the replicas when required. A Volume will be marked as online for Read and Write - if at least more than 51% of the Replicas are online. 

The number of replicas and other attributes of the Jiva volumes can be configured via Storage Class - OpenEBS annotations. 

*For further info, checkout [OpenEBS Documentation](https://docs.openebs.io/) or join the [slack#](https://slack.openebs.io)*

## Inspiration and Credit

[Rancher Longhorn](https://github.com/rancher/longhorn-engine), which helped with the significant portion of the code in this repo, helped us to validate that Storage controllers can be run as microservices and they can be coded in Go. The iSCSI functionality is derived from [gostor/gotgt](https://github.com/gostor/gotgt).

## License

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fopenebs%2Fjiva.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fopenebs%2Fjiva?ref=badge_large)
