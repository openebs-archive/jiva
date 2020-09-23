/*
 This file was originally authored by Rancher Labs
 under Apache License 2018.

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

package app

import (
	"os"
	"os/signal"
	"syscall"
)

var (
	hooks = []func(){}
)

func addShutdown(f func()) {
	if len(hooks) == 0 {
		registerShutdown()
	}

	hooks = append(hooks, f)
}

func registerShutdown() {
	c := make(chan os.Signal, 1024)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for range c {
			for _, hook := range hooks {
				hook()
			}
			os.Exit(1)
		}
	}()
}
