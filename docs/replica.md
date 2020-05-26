# Replica

This document covers the details about replica initialization and its code flow. 

## Main

main.go file exists in the root directory which has a list of commands to do some operations or configure the replica. ReplicaCmd() is the function which calls startReplica() where it starts a http server, a tcp server and a agent http server. Cope snnipets are given below:
```
// main.go
func  longhornCli() {
......... skipped ...........
	a.Commands = []cli.Command{
		app.ControllerCmd(),
		app.ReplicaCmd(),
		app.SyncAgentCmd(),
		app.AddReplicaCmd(),
		app.LsReplicaCmd(),
		app.RmReplicaCmd(),
		app.SnapshotCmd(),
		app.LogCmd(),
		app.SyncInfoCmd(),
		app.BackupCmd(),
		app.Journal(),
	}
........ skipped .........
	if  err := a.Run(os.Args); err != nil {
		logrus.Fatal("Error when executing command: ", err)
	}
}
// app/replica.go
func  startReplica(c *cli.Context) error {
........ skipped .........
	s := replica.NewServer(dir, 512, replicaType)
	// start a goroutine for freeing the space if there
	// are duplicate data.
	go replica.CreateHoles()
........ skipped .........
	// s is the global data structure and provides API'S for
	// reading/writing/updating data as well as management of
	// various operations on replicas like snapshot creation,
	// deletion, updation of metadata etc.
	//
	// When replica comes up for the first time, it creates data
	// files, their respective metafiles and some other metafiles
	// required for persisting replica's state. From the next restart
	// of replica it simply initializes the structure s only, rest
	// fields of s is initialized after getting open/start action from
	// controller.
	if  err := s.Create(size); err != nil {
		return err
	}
........ skipped .........
	// This goroutine is responsible for autoregistration of replica
	// and managing rebuild operations.
    go  AutoConfigureReplica(s, frontendIP, "tcp://"+address, replicaType)
	for s.Replica() == nil {
		logrus.Infof("Waiting for s.Replica() to be non nil")
		time.Sleep(2 * time.Second)
	}
........ skipped .........
```
