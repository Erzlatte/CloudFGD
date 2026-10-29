// Package sources contiene los conectores a las APIs públicas.
// Cada conector traduce la respuesta de su API al formato común Result.
package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Result es un resultado de búsqueda normalizado, igual para todas las fuentes.
type Result struct {
	Source     string            `json:"source"`
	ExternalID string            `json:"external_id"`
	Title      string            `json:"title"`
	Summary    string            `json:"summary"`
	Date       string            `json:"date"`
	URL        string            `json:"url"`
	Extra      map[string]string `json:"extra,omitempty"`
}

// Source es lo que tiene que cumplir cualquier conector.
type Source interface {
	Name() string
	Label() string
	Search(ctx context.Context, query string, limit int) ([]Result, error)
}

// ErrUpstream indica que la API externa falló o devolvió algo inesperado.
var ErrUpstream = errors.New("la fuente externa no respondió correctamente")

const userAgent = "CloudFGD/1.0 (proyecto educativo)"

// getJSON hace un GET y devuelve el cuerpo si la respuesta es 200.
func getJSON(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 5<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: estado HTTP %d", ErrUpstream, res.StatusCode)
	}
	return body, nil
}

func newClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

// truncate corta un texto a n caracteres sin partir letras con tilde.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n])) + "…"
}

var unsafeChars = regexp.MustCompile(`[^\p{L}\p{N}\s/\-.]`)

// cleanQuery quita caracteres que podrían romper la sintaxis de búsqueda de las APIs.
func cleanQuery(q string) string {
	q = unsafeChars.ReplaceAllString(q, " ")
	return strings.Join(strings.Fields(q), " ")
}
