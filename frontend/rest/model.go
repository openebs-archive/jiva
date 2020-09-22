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

package rest

import (
	"encoding/base64"

	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

type Volume struct {
	client.Resource
	Name string `json:"name"`
}

type ReadInput struct {
	client.Resource
	Offset int64 `json:"offset"`
	Length int64 `json:"length"`
}

type ReadOutput struct {
	client.Resource
	Data string `json:"data"`
}

type WriteInput struct {
	client.Resource
	Offset int64  `json:"offset"`
	Length int    `json:"length"`
	Data   string `json:"data"`
}

type WriteOutput struct {
	client.Resource
}

func NewVolume(context *api.ApiContext, name string) *Volume {
	v := &Volume{
		Resource: client.Resource{
			Id:      EncodeID(name),
			Type:    "volume",
			Actions: map[string]string{},
		},
		Name: name,
	}

	v.Actions["readat"] = context.UrlBuilder.ActionLink(v.Resource, "readat")
	v.Actions["writeat"] = context.UrlBuilder.ActionLink(v.Resource, "writeat")
	return v
}

func DecodeID(id string) (string, error) {
	b, err := DecodeData(id)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func EncodeID(id string) string {
	return EncodeData([]byte(id))
}
func DecodeData(data string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func EncodeData(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func NewSchema() *client.Schemas {
	schemas := &client.Schemas{}

	schemas.AddType("error", client.ServerApiError{})
	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})
	schemas.AddType("readInput", ReadInput{})
	schemas.AddType("readOutput", ReadOutput{})
	schemas.AddType("writeInput", WriteInput{})
	schemas.AddType("writeOutput", WriteOutput{})

	volumes := schemas.AddType("volume", Volume{})
	volumes.ResourceActions = map[string]client.Action{
		"readat": {
			Input:  "readInput",
			Output: "readOutput",
		},
		"writeat": {
			Input:  "writeInput",
			Output: "writeOutput",
		},
	}

	return schemas
}

type Server struct {
	d *Device
}

func NewServer(d *Device) *Server {
	return &Server{
		d: d,
	}
}
