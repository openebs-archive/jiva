package app

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func DelVolumeCmd() cli.Command {
	return cli.Command{
		Name:      "del-replica",
		ShortName: "del",
		Action: func(c *cli.Context) {
			if err := deleteVolume(c); err != nil {
				logrus.Fatalf("Error running del replica command: %v", err)
			}
		},
	}
}

// deleteVolume is used to delete all the contents of all the
// replicas. In case a volume is not found it returns error else
// it prints the details of the deleted replicas and any error if exist.
func deleteVolume(c *cli.Context) error {
	controllerClient := getCli(c)
	out, err := controllerClient.DeleteVolume()
	if err != nil {
		return err
	}
	format := "%s\t%s\t%5s\n"
	tw := tabwriter.NewWriter(os.Stdout, 0, 20, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(tw, format, "ADDRESS", "MESSAGE", "ERROR")
	for _, val := range out.DeletedReplicasInfo {
		fmt.Fprintf(tw, format, val.Replica, val.Msg, val.Error)
	}
	tw.Flush()
	return nil
}
