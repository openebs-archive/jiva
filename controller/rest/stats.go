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

package rest

import (
	"net/http"

	journal "github.com/openebs/sparse-tools/stats"
	"github.com/rancher/go-rancher/api"
)

//ListJournal flushes operation journal (replica read/write, ping, etc.) accumulated since previous flush
func (s *Server) ListJournal(rw http.ResponseWriter, req *http.Request) error {
	var input JournalInput
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil {
		return err
	}
	journal.PrintLimited(input.Limit)
	return nil
}
