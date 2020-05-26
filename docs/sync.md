## Sync (Rebuilding)

Once replica is successfully added to controller in WO mode, it goes for rebuilding process. If revision count and chain length of healthy replica (RW) and current replica is same then rebuilding is skipped and notified to controller to make it RW in reloadAndVerify function in sync/sync.go else it goes for full rebuild as explained below:

- WO replica fetches the chain from the RW replica and rebuild in order from oldest to latest and head file is skipped (since IO's are being written to it). For example if the chain at RW replica is H=>E=>D=>C=>B=>A=>" " , then A will be synced first the B and so on.
- Sync server (WO replica) and Sync client (RW) is started by calling sync-agent for the sync of each snapshot. See function syncFiles and sync/agent/process.go for more details.
- Sync agent launch the client and server as child process and poll for it's status every 2 second.
- Please go through [sparse-tools]([https://github.com/openebs/sparse-tools](https://github.com/openebs/sparse-tools)) to know more details about the sync process 



