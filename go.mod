module github.com/openebs/jiva

go 1.13

require (
	github.com/docker/docker v1.13.1
	github.com/docker/go-units v0.4.0
	github.com/frostschutz/go-fibmap v0.0.0-20160825162329-b32c231bfe6a
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/mux v1.7.4
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/gostor/gotgt v0.1.1-0.20191128095459-2f1d32710a93
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/openebs/sparse-tools v1.0.0
	github.com/prometheus/client_golang v1.5.1
	github.com/rancher/go-rancher v0.1.1-0.20190307222549-9756097e5e4c
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/urfave/cli v1.22.3
	go.uber.org/zap v1.14.1
	golang.org/x/sys v0.0.0-20200302150141-5c8b2ff67527
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
)

replace github.com/frostschutz/go-fibmap => github.com/rancher/go-fibmap v0.0.0-20160418233256-5fc9f8c1ed47
