package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Message es un mensaje del libro de visitas.
type Message struct {
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Store guarda los mensajes en memoria y los persiste en un fichero JSON
// dentro del volumen de Docker. Un mutex protege el acceso concurrente,
// ya que el servidor HTTP atiende cada petición en su propia goroutine.
type Store struct {
	mu   sync.Mutex
	path string
	msgs []Message
}

// NewStore carga los mensajes existentes (si el fichero ya existe).
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil // primera ejecución: no hay datos todavía
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &s.msgs); err != nil {
			return nil, fmt.Errorf("fichero de datos corrupto: %w", err)
		}
	}
	return s, nil
}

// Add añade un mensaje y lo escribe en disco de forma atómica
// (fichero temporal + rename), así un corte a mitad de escritura
// nunca deja el JSON a medias.
func (s *Store) Add(m Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	msgs := append(s.msgs, m)
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.msgs = msgs
	return nil
}

// Latest devuelve los últimos n mensajes, del más reciente al más antiguo.
func (s *Store) Latest(n int) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Message, 0, n)
	for i := len(s.msgs) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, s.msgs[i])
	}
	return out
}

// Writable comprueba que el directorio de datos admite escrituras.
// Lo usa /healthz: si el volumen no está montado o es de solo lectura,
// el contenedor se marca como "unhealthy".
func (s *Store) Writable() error {
	f, err := os.CreateTemp(filepath.Dir(s.path), ".healthcheck-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}
