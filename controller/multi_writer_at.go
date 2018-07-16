package controller

import (
	"io"
	"strings"
	"sync"
)

type MultiWriterAt struct {
	writers  []io.WriterAt
	updaters []io.WriterAt
}

type MultiWriterError struct {
	Writers       []io.WriterAt
	Updaters      []io.WriterAt
	ReplicaErrors []error
	QuorumErrors  []error
}

func (m *MultiWriterError) Error() string {
	errors := []string{}
	for _, err := range m.ReplicaErrors {
		if err != nil {
			errors = append(errors, err.Error())
		}
	}

	for _, err := range m.QuorumErrors {
		if err != nil {
			errors = append(errors, err.Error())
		}
	}

	switch len(errors) {
	case 0:
		return "Unknown"
	case 1:
		return errors[0]
	default:
		return strings.Join(errors, "; ")
	}
}

func (m *MultiWriterAt) WriteAt(p []byte, off int64) (n int, err error) {
	quorumErrs := make([]error, len(m.updaters))
	replicaErrs := make([]error, len(m.writers))
	replicaErrCount := 0
	quorumErrCount := 0
	quorumErrored := false
	replicaErrored := false
	wg := sync.WaitGroup{}
	var errors MultiWriterError
	var multiWriterMtx sync.Mutex

	multiWriterMtx = sync.Mutex{}

	for i, w := range m.writers {
		wg.Add(1)
		go func(index int, w io.WriterAt) {
			_, err := w.WriteAt(p, off)
			if err != nil {
				multiWriterMtx.Lock()
				replicaErrored = true
				replicaErrs[index] = err
				multiWriterMtx.Unlock()
			}
			wg.Done()
		}(i, w)
	}
	for i, w := range m.updaters {
		wg.Add(1)
		go func(index int, w io.WriterAt) {
			_, err := w.WriteAt(nil, 0)
			if err != nil {
				multiWriterMtx.Lock()
				quorumErrored = true
				quorumErrs[index] = err
				multiWriterMtx.Unlock()
			}
			wg.Done()
		}(i, w)
	}
	wg.Wait()
	if replicaErrored {
		errors.Writers = m.writers
		errors.ReplicaErrors = replicaErrs
	} else if quorumErrored {
		errors.Updaters = m.updaters
		errors.QuorumErrors = quorumErrs
	}
	//Below code is introduced to make sure that the IO has been written to more
	//than 50% of the replica.
	//If any replica has errored, return with the length of data written and the
	//erroed replica details so that it can be closed.
	if replicaErrored || quorumErrored {
		for _, err1 := range replicaErrs {
			if err1 != nil {
				replicaErrCount++
			}
		}
		for _, err1 := range quorumErrs {
			if err1 != nil {
				quorumErrCount++
			}
		}
		if (len(m.writers)-replicaErrCount > len(m.writers)/2) &&
			(len(m.writers)+len(m.updaters)-replicaErrCount-quorumErrCount > (len(m.writers)+len(m.updaters))/2) {
			return len(p), &errors
		}
		return 0, &errors
	}
	return len(p), nil
}
