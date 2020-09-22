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
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

func (s *Server) ListVolumes(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	apiContext.Write(&client.GenericCollection{
		Data: []interface{}{
			s.listVolumes(apiContext)[0],
		},
	})
	return nil
}

func (s *Server) GetVolume(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	v := s.getVolume(apiContext, id)
	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	apiContext.Write(v)
	return nil
}

func (s *Server) ReadAt(rw http.ResponseWriter, req *http.Request) error {
	var input ReadInput

	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	v := s.getVolume(apiContext, id)
	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	if err := apiContext.Read(&input); err != nil {
		return err
	}

	buf := make([]byte, input.Length)
	_, err := s.d.backend.ReadAt(buf, input.Offset)
	if err != nil {
		log.Errorln("read failed: ", err.Error())
		return fmt.Errorf("read failed: %v", err.Error())
	}

	data := EncodeData(buf)
	apiContext.Write(&ReadOutput{
		Resource: client.Resource{
			Type: "readOutput",
		},
		Data: data,
	})
	return nil
}

func (s *Server) WriteAt(rw http.ResponseWriter, req *http.Request) error {
	var input WriteInput

	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	v := s.getVolume(apiContext, id)
	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	if err := apiContext.Read(&input); err != nil {
		return err
	}

	buf, err := DecodeData(input.Data)
	if err != nil {
		return err
	}
	if len(buf) != input.Length {
		return fmt.Errorf("Inconsistent length in request")
	}

	if _, err := s.d.backend.WriteAt(buf, input.Offset); err != nil {
		log.Errorln("write failed: ", err.Error())
		return err
	}
	apiContext.Write(&WriteOutput{
		Resource: client.Resource{
			Type: "writeOutput",
		},
	})
	return nil
}

func (s *Server) listVolumes(context *api.ApiContext) []*Volume {
	return []*Volume{
		NewVolume(context, s.d.Name),
	}
}

func (s *Server) getVolume(context *api.ApiContext, id string) *Volume {
	for _, v := range s.listVolumes(context) {
		if v.Id == id {
			return v
		}
	}
	return nil
}
