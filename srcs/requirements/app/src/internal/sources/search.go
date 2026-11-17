package sources

import (
	"context"
	"errors"
	"sync"
)

type SourceError struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

func SearchAll(ctx context.Context, srcs []Source, query string, limit int) ([]Result, []SourceError) {
	type answer struct {
		results []Result
		err     error
	}
	answers := make([]answer, len(srcs))

	var wg sync.WaitGroup
	for i, s := range srcs {
		wg.Add(1)
		go func(i int, s Source) {
			defer wg.Done()
			r, err := s.Search(ctx, query, limit)
			answers[i] = answer{r, err}
		}(i, s)
	}
	wg.Wait()

	results := []Result{}
	errs := []SourceError{}
	for i, a := range answers {
		if a.err != nil {
			errs = append(errs, SourceError{Source: srcs[i].Name(), Message: userMessage(a.err)})
			continue
		}
		results = append(results, a.results...)
	}
	return results, errs
}

func userMessage(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "la fuente tardó demasiado en responder"
	case errors.Is(err, ErrUpstream):
		return err.Error()
	default:
		return "error inesperado al consultar la fuente"
	}
}
