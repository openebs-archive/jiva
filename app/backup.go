package app

import (
	"fmt"

	"github.com/openebs/jiva/sync"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func BackupCmd() cli.Command {
	return cli.Command{
		Name:      "backups",
		ShortName: "backup",
		Subcommands: []cli.Command{
			BackupCreateCmd(),
			BackupRestoreCmd(),
			BackupRmCmd(),
			BackupInspectCmd(),
			BackupListCmd(),
		},
	}
}

func BackupCreateCmd() cli.Command {
	return cli.Command{
		Name:  "create",
		Usage: "create a backup in objectstore: create <snapshot> --dest <dest>",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "dest",
				Usage: "destination of backup if driver supports, would be url like s3://bucket@region/path/ or vfs:///path/",
			},
		},
		Action: func(c *cli.Context) {
			if err := createBackup(c); err != nil {
				logrus.Fatalf("Error running create backup command: %v", err)
			}
		},
	}
}

func BackupRmCmd() cli.Command {
	return cli.Command{
		Name:  "rm",
		Usage: "remove a backup in objectstore: rm <backup>",
		Action: func(c *cli.Context) {
			if err := rmBackup(c); err != nil {
				logrus.Fatalf("Error running rm backup command: %v", err)
			}
		},
	}
}

func BackupRestoreCmd() cli.Command {
	return cli.Command{
		Name:  "restore",
		Usage: "restore a backup to current volume: restore <backup>",
		Action: func(c *cli.Context) {
			if err := restoreBackup(c); err != nil {
				logrus.Fatalf("Error running restore backup command: %v", err)
			}
		},
	}
}

func BackupInspectCmd() cli.Command {
	return cli.Command{
		Name:  "inspect",
		Usage: "inspect a backup: inspect <backup>",
		Action: func(c *cli.Context) {
			if err := inspectBackup(c); err != nil {
				logrus.Fatalf("Error running inspect backup command: %v", err)
			}
		},
	}
}

func BackupListCmd() cli.Command {
	return cli.Command{
		Name:      "list",
		ShortName: "ls",
		Usage:     "list backup: list <dest>",
		Action: func(c *cli.Context) {
			if err := listBackup(c); err != nil {
				logrus.Fatalf("Error running inspect backup command: %v", err)
			}
		},
	}
}

func createBackup(c *cli.Context) error {
	url := c.GlobalString("url")
	task := sync.NewTask(url)

	dest := c.String("dest")
	if dest == "" {
		return fmt.Errorf("Missing required parameter --dest")
	}

	snapshot := c.Args().First()
	if snapshot == "" {
		return fmt.Errorf("Missing required parameter snapshot")
	}

	backup, err := task.CreateBackup(snapshot, dest)
	if err != nil {
		return err
	}
	fmt.Println(backup)

	return nil
}

func rmBackup(c *cli.Context) error {
	url := c.GlobalString("url")
	task := sync.NewTask(url)

	backup := c.Args().First()
	if backup == "" {
		return fmt.Errorf("Missing required parameter backup")
	}

	return task.RmBackup(backup)
}

func restoreBackup(c *cli.Context) error {
	url := c.GlobalString("url")
	task := sync.NewTask(url)

	backup := c.Args().First()
	if backup == "" {
		return fmt.Errorf("Missing required parameter backup")
	}

	return task.RestoreBackup(backup)
}

func inspectBackup(c *cli.Context) error {
	url := c.GlobalString("url")
	task := sync.NewTask(url)

	backup := c.Args().First()
	if backup == "" {
		return fmt.Errorf("Missing required parameter backup")
	}

	output, err := task.InspectBackup(backup)
	if err != nil {
		return err
	}
	fmt.Println(output)

	return nil
}

func listBackup(c *cli.Context) error {
	url := c.GlobalString("url")
	task := sync.NewTask(url)

	destURL := c.Args().First()
	if destURL == "" {
		return fmt.Errorf("Missing required parameter <dest>")
	}

	output, err := task.ListBackup(destURL)
	if err != nil {
		return err
	}
	fmt.Println(output)

	return nil
}
