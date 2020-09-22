/*
 Copyright © 2020 The OpenEBS Authors

 This file was originally authored by Rancher Labs
 under Apache License 2018.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/openebs/jiva/sync/agent"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func SyncAgentCmd() cli.Command {
	return cli.Command{
		Name:      "sync-agent",
		UsageText: "longhorn controller DIRECTORY SIZE",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "listen",
				Value: "localhost:9504",
			},
			cli.StringFlag{
				Name:  "listen-port-range",
				Value: "9700-9800",
			},
		},
		Action: func(c *cli.Context) {
			if err := startSyncAgent(c); err != nil {
				logrus.Fatalf("Error running sync-agent command: %v", err)
			}
		},
	}
}

func startSyncAgent(c *cli.Context) error {
	listen := c.String("listen")
	portRange := c.String("listen-port-range")

	parts := strings.Split(portRange, "-")
	if len(parts) != 2 {
		return fmt.Errorf("Invalid format for range: %s", portRange)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return err
	}

	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return err
	}

	server := agent.NewServer(start, end)
	router := agent.NewRouter(server)
	logrus.Infof("Listening on sync %s start: %d end: %d", listen, start, end)

	return http.ListenAndServe(listen, router)
}
