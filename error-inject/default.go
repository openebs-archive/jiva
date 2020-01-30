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

// AddWriteTimeout add delay in while writing
func AddWriteTimeout() {}

// DisablePunchHoles is used for disabling punch holes
func DisablePunchHoles() bool { return false }

// PanicAfterPrepareRebuild is used for crashing the replica
// just after prepare rebuild.
func PanicAfterPrepareRebuild() {}
