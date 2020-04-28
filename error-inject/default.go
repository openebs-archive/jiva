// +build !debug

package inject

// AddTimeout add delays into the code
func AddTimeout() {}

// AddPingTimeout add delay in ping response
func AddPingTimeout() {}

// AddPreloadTimeout add delay in preload
func AddPreloadTimeout() {}

// AddPunchHoleTimeout add delay in while punching hole
func AddPunchHoleTimeout() {}

// DisablePunchHoles is used for disabling punch holes
func DisablePunchHoles() bool { return false }

// PanicAfterPrepareRebuild is used for crashing the replica
// just after prepare rebuild.
func PanicAfterPrepareRebuild() {}

// UpdateLUNMapTimeoutTriggered is being used to wait for the delay in
// UpdateLUNMap to start
var UpdateLUNMapTimeoutTriggered bool

// AddUpdateLUNMapTimeout adds delay during UpdateLUNMap
func AddUpdateLUNMapTimeout() {}
