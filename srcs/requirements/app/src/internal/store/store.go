package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type SavedItem struct {
	ID         string            `json:"id"`
	Source     string            `json:"source"`
	ExternalID string            `json:"external_id"`
	Title      string            `json:"title"`
	Summary    string            `json:"summary"`
	Date       string            `json:"date"`
	URL        string            `json:"url"`
	Extra      map[string]string `json:"extra,omitempty"`
	Tags       []string          `json:"tags"`
	Note       string            `json:"note"`
	SavedAt    time.Time         `json:"saved_at"`
}

type Filter struct {
	Source string
	Tag    string
	From   string
	To     string
}

var (
	ErrDuplicate = errors.New("el elemento ya está guardado")
	ErrNotFound  = errors.New("el elemento no existe")
)

type Store struct {
	mu    sync.Mutex
	path  string
	items []SavedItem
}

func New(path string) (*Store, error) {
	s := &Store{path: path}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &s.items); err != nil {
			return nil, fmt.Errorf("fichero de datos corrupto: %w", err)
		}
	}
	return s, nil
}

func (s *Store) Add(it SavedItem) (SavedItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, old := range s.items {
		if old.Source == it.Source && old.ExternalID == it.ExternalID {
			return old, ErrDuplicate
		}
	}

	id, err := newID()
	if err != nil {
		return SavedItem{}, err
	}
	it.ID = id
	it.SavedAt = time.Now().UTC().Truncate(time.Second)

	items := append(append([]SavedItem{}, s.items...), it)
	if err := s.persist(items); err != nil {
		return SavedItem{}, err
	}
	s.items = items
	return it, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, it := range s.items {
		if it.ID != id {
			continue
		}
		items := append(append([]SavedItem{}, s.items[:i]...), s.items[i+1:]...)
		if err := s.persist(items); err != nil {
			return err
		}
		s.items = items
		return nil
	}
	return ErrNotFound
}

func (s *Store) List(f Filter) []SavedItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := []SavedItem{}
	for i := len(s.items) - 1; i >= 0; i-- {
		if f.matches(s.items[i]) {
			out = append(out, s.items[i])
		}
	}
	return out
}

func (s *Store) Writable() error {
	f, err := os.CreateTemp(filepath.Dir(s.path), ".healthcheck-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

func (s *Store) persist(items []SavedItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (f Filter) matches(it SavedItem) bool {
	if f.Source != "" && it.Source != f.Source {
		return false
	}
	if f.Tag != "" && !contains(it.Tags, f.Tag) {
		return false
	}
	if (f.From != "" || f.To != "") && it.Date == "" {
		return false
	}
	if f.From != "" && it.Date < f.From {
		return false
	}
	if f.To != "" && it.Date > f.To {
		return false
	}
	return true
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
