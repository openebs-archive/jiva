// +build !debug

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

package inject

var Envs map[string](map[string]bool)

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

// PanicWhileSettingCheckpoint is used for crashing the replica
// on receiving set checkpoint REST Call
func PanicWhileSettingCheckpoint(addr string) {}

// UpdateLUNMapTimeoutTriggered is being used to wait for the delay in
// UpdateLUNMap to start
var UpdateLUNMapTimeoutTriggered bool

// AddUpdateLUNMapTimeout adds delay during UpdateLUNMap
func AddUpdateLUNMapTimeout() {}
