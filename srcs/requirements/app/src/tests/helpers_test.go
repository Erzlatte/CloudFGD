package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"CloudFGD/app/internal/api"
	"CloudFGD/app/internal/sources"
	"CloudFGD/app/internal/store"
	"CloudFGD/app/web"
)

type capturedRequest struct {
	req *http.Request
}

func fakeAPI(t *testing.T, status int, file string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	last := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last.req = r.Clone(context.Background())
		w.WriteHeader(status)
		if file == "" {
			return
		}
		data, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, last
}

type stubSource struct {
	name    string
	results []sources.Result
	err     error
	delay   time.Duration
}

func (s stubSource) Name() string  { return s.name }
func (s stubSource) Label() string { return "Fuente " + s.name }

func (s stubSource) Search(ctx context.Context, q string, limit int) ([]sources.Result, error) {
	select {
	case <-time.After(s.delay):
		return s.results, s.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type testApp struct {
	handler   http.Handler
	storePath string
}

func newTestApp(t *testing.T, srcs ...sources.Source) testApp {
	t.Helper()
	path := filepath.Join(t.TempDir(), "saved.json")
	st, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	h := api.New(api.Deps{
		Store:         st,
		Sources:       srcs,
		SearchTimeout: time.Second,
		Static:        web.Files(),
	})
	return testApp{handler: h, storePath: path}
}

func (a testApp) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}
