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
