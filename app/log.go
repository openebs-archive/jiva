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
