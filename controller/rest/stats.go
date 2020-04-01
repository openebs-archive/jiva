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
