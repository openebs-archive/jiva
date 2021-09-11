# Jiva

[![Build Status](https://github.com/openebs/jiva/actions/workflows/build.yaml/badge.svg)](https://github.com/openebs/jiva/actions/workflows/build.yml)
[![Releases](https://img.shields.io/github/release/openebs/jiva/all.svg?style=flat-square)](https://github.com/openebs/jiva/releases)
[![Docker Pulls](https://img.shields.io/docker/pulls/openebs/jiva)](https://hub.docker.com/repository/docker/openebs/jiva)
[![Slack](https://img.shields.io/badge/chat!!!-slack-ff1493.svg?style=flat-square)](https://kubernetes.slack.com/messages/openebs)
[![Community Meetings](https://img.shields.io/badge/Community-Meetings-blue)](https://hackmd.io/hiRcXyDTRVO2_Zs9fp0CAg)
[![Twitter](https://img.shields.io/twitter/follow/openebs.svg?style=social&label=Follow)](https://twitter.com/intent/follow?screen_name=openebs)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](/CONTRIBUTING.md)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/616f61627a4543febe14af30358805b9)](https://www.codacy.com/app/OpenEBS/jiva?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=openebs/jiva&amp;utm_campaign=Badge_Grade)
[![Go Report Card](https://goreportcard.com/badge/github.com/openebs/jiva)](https://goreportcard.com/report/github.com/openebs/jiva)
[![GoDoc](https://godoc.org/github.com/openebs/jiva?status.svg)](https://godoc.org/github.com/openebs/jiva)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1755/badge)](https://bestpractices.coreinfrastructure.org/projects/1755)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fopenebs%2Fjiva.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fopenebs%2Fjiva?ref=badge_shield)


<img width="200" align="right" alt="OpenEBS Logo" src="https://raw.githubusercontent.com/cncf/artwork/HEAD/projects/openebs/stacked/color/openebs-stacked-color.png" xmlns="http://www.w3.org/1999/html">

<p align="justify">
<strong>OpenEBS Jiva</strong> can be used to dynamically provision highly available Kubernetes Persistent Volumes using local (ephemeral) storage available on the Kubernetes nodes. 
</p>
<br>

## Overview

Jiva provides containerized block storage controller. 

[Jiva Operator](https://github.com/openebs/jiva-operator) helps with dynamically provisioning Jiva Volumes and managing their lifecycle. 

## Usage

- Refer to the [Jiva Operator Quickstart guide](https://github.com/openebs/jiva-operator).

## Supported Block Storage Features

- [x] Thin Provisioning
- [x] Enforce volume quota
- [x] Synchronous replication
- [x] High Availability
- [ ] Incremental/Full Snapshot 
- [x] Full Backup and Restore (using Velero)
- [x] Supports OS/ARCH: linux/arm64, linux/amd64

### How does High Availability work?

Jiva comprises of two components:
-   A Target ( or a Storage Controller) that exposes iSCSI, while synchronously replicating the data to one or more Replicas. 
-   A set of Replicas that a Target uses to read/write data. Each of the replicas will be on a different node to ensure high availability against node or network failures. The Replicas save the data into sparse files on the host filesystem directories. 

For ensuring that jiva volumes can sustain a node failure, the volumes must be configured with atleast 3 replicas. The target will serve the data as long as there are two healthy replicas. When a node with a replica fails and comes back online, the replica will start in a degraded mode and start rebuilding the missed data from the available healthy replicas. 

If 2 of the 3 replicas go offline at the same time, then volume will be marked as read-only. This ensures that target can decided which of the replicas has the latest data.

### How does Jiva Perform?

Jiva is optimized for consistency and availability and not performance. There is a significant degradation in performance compared to local storage due to the following design choices:
- Jiva uses "strong consistency" approach where target will make the write operation as completed only after the data is confirmed to be written to all the healthy replicas. 
- Also Jiva does not implement any caching logic, to allow for the jiva target and replica containers to be ephemeral. 
- Jiva engine is completely written in user space to be able to run on any platform and make upgrades seamless.

If you need low latency storage, please checkout other OpenEBS Engines like [Mayastor and Local PVs](https://openebs.io/docs/concepts/casengines).

# Who uses Jiva?

Some of the organizations that have publicly shared the usage of Jiva.
- [Arista](https://github.com/openebs/openebs/blob/HEAD/adopters/arista/README.md)
- [CLEW Medical](https://github.com/openebs/openebs/blob/HEAD/adopters/clewmedical/README.md)
- [Clouds Sky GmbH](https://github.com/openebs/openebs/blob/HEAD/adopters/cloudssky/README.md)
- [CodeWave](https://github.com/openebs/openebs/blob/HEAD/adopters/codewave/README.md)
- [Verizon Media](https://github.com/openebs/openebs/blob/HEAD/adopters/verizon/README.md)

You can see the full list of organizations/users using OpenEBS [here](https://github.com/openebs/openebs/blob/HEAD/ADOPTERS.md).

If you are using Jiva, and would like yourself to be listed in this page as a an adopter, please raise an [issue](https://github.com/openebs/jiva-operator/issues/new?assignees=&labels=&template=become-an-adopter.md&title=%5BADOPTER%5D) with the following details:

- Stateful Applications that you are running on OpenEBS
- Are you evaluating or already using in development, CI/CD, production
- Are you using it for home use or for your organization
- A brief description of the use case or details on how OpenEBS is helping your projects.

If you would like your name (as home user) or organization name to be added to the [ADOPTERS.md](https://github.com/openebs/openebs/blob/HEAD/ADOPTERS.md), please provide a preferred contact handle like github id, twitter id, linkedin id, website etc.

## Contributing

OpenEBS welcomes your feedback and contributions in any form possible.

- [Join OpenEBS community on Kubernetes Slack](https://kubernetes.slack.com)
  - Already signed up? Head to our discussions at [#openebs](https://kubernetes.slack.com/messages/openebs/)
- Want to raise an issue or help with fixes and features?
  - See [open issues](https://github.com/openebs/jiva-operator/issues)
  - See [contributing guide](./CONTRIBUTING.md)
  - See [Project Roadmap](https://github.com/openebs/openebs/blob/HEAD/ROADMAP.md#jiva)
  - Want to join our contributor community meetings, [check this out](https://hackmd.io/mfG78r7MS86oMx8oyaV8Iw?view).
- Join our OpenEBS CNCF Mailing lists
  - For OpenEBS project updates, subscribe to [OpenEBS Announcements](https://lists.cncf.io/g/cncf-openebs-announcements)
  - For interacting with other OpenEBS users, subscribe to [OpenEBS Users](https://lists.cncf.io/g/cncf-openebs-users)

## Code of conduct

Please read the community code of conduct [here](./CODE_OF_CONDUCT.md).


## Inspiration and Credit
OpenEBS Jiva uses the following two projects:
- Fork of the [Longhorn engine](https://github.com/longhorn/longhorn-engine), which helped with the significant portion of the code in this repo, helped us to validate that Storage controllers can be run as microservices and they can be coded in Go. 
- The iSCSI functionality is provided by [gostor/gotgt](https://github.com/gostor/gotgt).

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fopenebs%2Fjiva.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fopenebs%2Fjiva?ref=badge_large)
