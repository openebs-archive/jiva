package main

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func getJivaImageID() string {
	var jivaImageID string
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		panic(err)
	}

	for _, image := range images {
		if strings.Contains(image.RepoTags[0], "openebs/jiva") {
			jivaImageID = image.ID
		}
	}
	return jivaImageID
}

func getJivaDebugImageID() string {
	var jivaImageID string
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		panic(err)
	}

	for _, image := range images {
		if strings.Contains(image.RepoTags[0], "openebs/jiva") &&
			strings.Contains(image.RepoTags[0], "DEBUG") {
			jivaImageID = image.ID
		}
	}
	return jivaImageID
}

func createReplica(replicaIP string, config *TestConfig) string {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: config.Image,
			Cmd: func() []string {
				var envs []string
				for key, value := range config.ReplicaEnvs {
					pair := []string{key, value}
					envs = append(envs, pair...)
				}
				args := []string{
					"launch", "replica",
					"--frontendIP", config.ControllerIP,
					"--listen", replicaIP + ":9502",
					"--size", "5g", "/vol",
				}
				return append(envs, args...)
			}(),
			ExposedPorts: nat.PortSet{
				"9502/tcp": {},
				"9503/tcp": {},
				"9504/tcp": {},
			},
		},
		&container.HostConfig{
			RestartPolicy: container.RestartPolicy{
				Name:              "unless-stopped",
				MaximumRetryCount: 0,
			},
			NetworkMode:     "stg-net",
			PublishAllPorts: true,
			Binds:           []string{"/tmp/" + replicaIP + "vol:/vol"},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				"stg-net": &network.EndpointSettings{
					IPAMConfig: &network.EndpointIPAMConfig{
						IPv4Address: replicaIP,
					},
				},
			},
		}, "Replica_"+replicaIP,
	)
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	return resp.ID
}

func createController(controllerIP string, config *TestConfig) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: config.Image,
			Cmd:   []string{"env", "REPLICATION_FACTOR=" + config.ReplicationFactor, "launch", "controller", "--frontend", "gotgt", "--frontendIP", controllerIP, config.VolumeName},
			ExposedPorts: nat.PortSet{
				"3260/tcp": {},
				"9501/tcp": {},
			},
		},
		&container.HostConfig{
			NetworkMode:     "stg-net",
			PublishAllPorts: true,
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				"stg-net": &network.EndpointSettings{
					IPAMConfig: &network.EndpointIPAMConfig{
						IPv4Address: controllerIP,
					},
				},
			},
		}, "controller_"+controllerIP,
	)
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	config.Controller[controllerIP] = resp.ID
}

func deleteController(config *TestConfig) {
	stopContainer(config.Controller[config.ControllerIP])
	removeContainer(config.Controller[config.ControllerIP])
}

func stopContainer(containerID string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStop(ctx, containerID, nil); err != nil {
		panic(err)
	}
}

func removeContainer(containerID string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{}); err != nil {
		panic(err)
	}
}

func startContainer(containerID string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}
}

func verifyRestartCount(containerID string, restartCount int) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	containerInspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		panic(err)
	}
	for {
		if containerInspect.ContainerJSONBase.RestartCount >= restartCount {
			break
		}
	}
}
