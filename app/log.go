/*
 Copyright © 2020 The OpenEBS Authors

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
	"github.com/openebs/jiva/util"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func LogCmd() cli.Command {
	return cli.Command{
		Name: "logtofile",
		Subcommands: []cli.Command{
			LogEnable(),
			LogDisable(),
		},
		Action: func(c *cli.Context) {
		},
	}
}

func LogEnable() cli.Command {
	return cli.Command{
		Name: "enable",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  retentionPeriod,
				Usage: "Retention period of log file in days",
				Value: defaultRetentionPeriod,
			},
			cli.IntFlag{
				Name:  maxLogFileSize,
				Usage: "Max size of log file in mb",
				Value: defaultLogFileSize,
			},
			cli.IntFlag{
				Name:  maxBackups,
				Usage: "Max number of log files to keep while creating new log file once size of log exceeds to maxLogFileSize",
				Value: defaultMaxBackups,
			},
		},
		Action: func(c *cli.Context) {
			if err := setLogs(c, true); err != nil {
				logrus.Fatalf("Fail to enable logs, err: %v", err)
			}
		},
	}
}

func LogDisable() cli.Command {
	return cli.Command{
		Name: "disable",
		Action: func(c *cli.Context) {
			if err := setLogs(c, false); err != nil {
				logrus.Fatalf("Fail to enable logs, err: %v", err)
			}
		},
	}
}

func setLogs(c *cli.Context, enable bool) error {
	cli := getCli(c)
	if err := cli.SetLogging(util.LogToFile{
		Enable:          enable,
		MaxLogFileSize:  c.Int(maxLogFileSize),
		MaxBackups:      c.Int(maxBackups),
		RetentionPeriod: c.Int(retentionPeriod),
	}); err != nil {
		return err
	}
	return nil
}
