/*
 Copyright © 2020 The OpenEBS Authors

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

package dynamic

import (
	"fmt"
	"strings"

	"github.com/openebs/jiva/types"
)

type Factory struct {
	factories map[string]types.BackendFactory
}

func New(factories map[string]types.BackendFactory) types.BackendFactory {
	return &Factory{
		factories: factories,
	}
}

func (d *Factory) Create(address string) (types.Backend, error) {
	parts := strings.SplitN(address, "://", 2)

	if len(parts) == 2 {
		if factory, ok := d.factories[parts[0]]; ok {
			return factory.Create(parts[1])
		}
	}

	return nil, fmt.Errorf("Failed to find factory for %s", address)
}

func (d *Factory) SignalToAdd(address string, action string) error {
	if factory, ok := d.factories["tcp"]; ok {
		return factory.SignalToAdd(address, action)
	}
	return nil
}

func (d *Factory) VerifyReplicaAlive(address string) bool {
	if factory, ok := d.factories["tcp"]; ok {
		return factory.VerifyReplicaAlive(address)
	}
	return false
}
