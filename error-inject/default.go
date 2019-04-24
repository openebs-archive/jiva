// +build !debug

package inject

func AddTimeout()          {}
func AddPingTimeout()      {}
func AddPreloadTimeout()   {}
func AddPunchHoleTimeout() {}
func IsDebugBuild() bool   { return false }
