module github.com/openebs/jiva

go 1.13

require (
	github.com/containerd/containerd v1.5.18 // indirect
	github.com/docker/docker v17.12.0-ce-rc1.0.20200531234253-77e06fda0c94+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.4.0
	github.com/frostschutz/go-fibmap v0.0.0-20160825162329-b32c231bfe6a
	github.com/google/uuid v1.2.0
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/mux v1.7.4
	github.com/gostor/gotgt v0.2.1-0.20210817044456-e5d5366e2b59
	github.com/kubernetes-csi/csi-lib-iscsi v0.0.0-20200118015005-959f12c91ca8
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/openebs/sparse-tools v1.1.0
	github.com/prometheus/client_golang v1.7.1
	github.com/rancher/go-rancher v0.1.1-0.20190307222549-9756097e5e4c
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.8.1
	github.com/urfave/cli v1.22.3
	go.uber.org/zap v1.14.1
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f
)

replace github.com/frostschutz/go-fibmap => github.com/rancher/go-fibmap v0.0.0-20160418233256-5fc9f8c1ed47
