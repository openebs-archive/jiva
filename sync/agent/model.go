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

package agent

import (
	"time"

	"github.com/rancher/go-rancher/client"
)

type Process struct {
	client.Resource
	ProcessType string    `json:"processType"`
	SrcFile     string    `json:"srcFile"`
	DestFile    string    `json:"destfile"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	ExitCode    int       `json:"exitCode"`
	Output      string    `json:"output"`
	Created     time.Time `json:"created"`
}

type ProcessCollection struct {
	client.Collection
	Data []Process `json:"data"`
}

func NewSchema() *client.Schemas {
	schemas := &client.Schemas{}

	schemas.AddType("error", client.ServerApiError{})
	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})
	process := schemas.AddType("process", Process{})
	process.CollectionMethods = []string{"GET", "POST"}

	for _, name := range []string{"file", "host", "port"} {
		f := process.ResourceFields[name]
		f.Create = true
		process.ResourceFields[name] = f
	}

	return schemas
}
