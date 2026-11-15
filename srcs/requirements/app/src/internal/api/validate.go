package api

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"CloudFGD/app/internal/store"
)

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (s *Server) validateSave(req SaveRequest) (store.SavedItem, error) {
	it := store.SavedItem{
		Source:     strings.TrimSpace(req.Source),
		ExternalID: strings.TrimSpace(req.ExternalID),
		Title:      strings.TrimSpace(req.Title),
		Summary:    strings.TrimSpace(req.Summary),
		Date:       strings.TrimSpace(req.Date),
		URL:        strings.TrimSpace(req.URL),
		Note:       strings.TrimSpace(req.Note),
	}

	if _, ok := s.sourceByName(it.Source); !ok {
		return it, errors.New("fuente desconocida")
	}
	if it.ExternalID == "" || utf8.RuneCountInString(it.ExternalID) > MaxExternalID {
		return it, errors.New("falta el identificador externo o es demasiado largo")
	}
	if it.Title == "" || utf8.RuneCountInString(it.Title) > MaxTitleLen {
		return it, fmt.Errorf("el título es obligatorio y no puede pasar de %d caracteres", MaxTitleLen)
	}
	if utf8.RuneCountInString(it.Summary) > MaxSummaryLen {
		return it, fmt.Errorf("el resumen no puede pasar de %d caracteres", MaxSummaryLen)
	}
	if utf8.RuneCountInString(it.Note) > MaxNoteLen {
		return it, fmt.Errorf("la nota no puede pasar de %d caracteres", MaxNoteLen)
	}
	if it.Date != "" && !isoDate.MatchString(it.Date) {
		return it, errors.New("la fecha debe tener el formato AAAA-MM-DD")
	}
	if !isHTTPURL(it.URL) {
		return it, errors.New("el enlace oficial debe ser una URL http(s)")
	}

	tags, err := normalizeTags(req.Tags)
	if err != nil {
		return it, err
	}
	it.Tags = tags

	extra, err := validateExtra(req.Extra)
	if err != nil {
		return it, err
	}
	it.Extra = extra

	return it, nil
}

func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func validateExtra(in map[string]string) (map[string]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if len(in) > MaxExtraFields {
		return nil, errors.New("demasiados campos extra")
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if utf8.RuneCountInString(k) > MaxExtraKeyLen || utf8.RuneCountInString(v) > MaxExtraValLen {
			return nil, errors.New("campo extra demasiado largo")
		}
		out[k] = v
	}
	return out, nil
}

func normalizeTag(t string) string {
	return strings.Join(strings.Fields(strings.ToLower(t)), " ")
}

func normalizeTags(in []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range in {
		t = normalizeTag(t)
		if t == "" || seen[t] {
			continue
		}
		if utf8.RuneCountInString(t) > MaxTagLen {
			return nil, fmt.Errorf("cada etiqueta puede tener como mucho %d caracteres", MaxTagLen)
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) > MaxTags {
		return nil, fmt.Errorf("como mucho %d etiquetas", MaxTags)
	}
	return out, nil
}
