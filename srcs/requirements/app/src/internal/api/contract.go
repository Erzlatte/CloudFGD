package api

import (
	"CloudFGD/app/internal/sources"
	"CloudFGD/app/internal/store"
)

const (
	RouteSources   = "/api/sources"
	RouteSearch    = "/api/search"
	RouteSaved     = "/api/saved"
	RouteSavedItem = "/api/saved/{id}"
	RouteHealth    = "/healthz"
)

const (
	MaxQueryLen    = 100
	DefaultLimit   = 10
	MaxLimit       = 25
	MaxExternalID  = 200
	MaxTitleLen    = 500
	MaxSummaryLen  = 1000
	MaxNoteLen     = 1000
	MaxTags        = 10
	MaxTagLen      = 30
	MaxExtraFields = 10
	MaxExtraKeyLen = 50
	MaxExtraValLen = 300
	MaxBodyBytes   = 16 << 10
)

type SourceInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

type SearchResponse struct {
	Query   string                `json:"query"`
	Results []sources.Result      `json:"results"`
	Errors  []sources.SourceError `json:"errors"`
}

type SaveRequest struct {
	Source     string            `json:"source"`
	ExternalID string            `json:"external_id"`
	Title      string            `json:"title"`
	Summary    string            `json:"summary"`
	Date       string            `json:"date"`
	URL        string            `json:"url"`
	Extra      map[string]string `json:"extra"`
	Tags       []string          `json:"tags"`
	Note       string            `json:"note"`
}

type SavedItem = store.SavedItem

type ErrorResponse struct {
	Error string `json:"error"`
	ID    string `json:"id,omitempty"`
}
