package api

import (
	"io/fs"
	"net/http"
	"time"

	"CloudFGD/app/internal/sources"
	"CloudFGD/app/internal/store"
)

type Deps struct {
	Store         *store.Store
	Sources       []sources.Source
	SearchTimeout time.Duration
	Static        fs.FS
}

type Server struct {
	store         *store.Store
	sources       []sources.Source
	searchTimeout time.Duration
	static        fs.FS
}

func New(d Deps) http.Handler {
	s := &Server{
		store:         d.Store,
		sources:       d.Sources,
		searchTimeout: d.SearchTimeout,
		static:        d.Static,
	}
	return s.routes()
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	files := http.FileServer(http.FS(s.static))

	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.Handle("GET /css/", files)
	mux.Handle("GET /js/", files)

	mux.HandleFunc("GET "+RouteSources, s.handleSources)
	mux.HandleFunc("GET "+RouteSearch, s.handleSearch)
	mux.HandleFunc("GET "+RouteSaved, s.handleListSaved)
	mux.HandleFunc("POST "+RouteSaved, s.handleCreateSaved)
	mux.HandleFunc("DELETE "+RouteSavedItem, s.handleDeleteSaved)
	mux.HandleFunc("GET "+RouteHealth, s.handleHealth)
	mux.HandleFunc("GET "+RoutePod, s.handlePod)

	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, s.static, "html/index.html")
}

func (s *Server) sourceByName(name string) (sources.Source, bool) {
	for _, src := range s.sources {
		if src.Name() == name {
			return src, true
		}
	}
	return nil, false
}
