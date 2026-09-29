package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxAuthor   = 50
	maxContent  = 500
	maxMessages = 50
)

// validate limpia y comprueba los campos de un mensaje nuevo.
func validate(author, content string) (Message, error) {
	author = strings.TrimSpace(author)
	content = strings.TrimSpace(content)
	switch {
	case author == "" || content == "":
		return Message{}, errors.New("el nombre y el mensaje son obligatorios")
	case utf8.RuneCountInString(author) > maxAuthor:
		return Message{}, errors.New("el nombre es demasiado largo")
	case utf8.RuneCountInString(content) > maxContent:
		return Message{}, errors.New("el mensaje es demasiado largo")
	}
	return Message{Author: author, Content: content, CreatedAt: time.Now()}, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// GET /api/messages -> últimos mensajes en JSON (los pinta app.js).
func (a *App) listMessages(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.Latest(maxMessages))
}

// POST /api/messages -> crea un mensaje a partir de un JSON {author, content}.
func (a *App) createMessageJSON(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Author  string `json:"author"`
		Content string `json:"content"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON no válido"})
		return
	}
	m, err := validate(in.Author, in.Content)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := a.store.Add(m); err != nil {
		log.Println("error guardando mensaje:", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo guardar el mensaje"})
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

// POST /messages -> alternativa sin JavaScript (formulario HTML clásico).
func (a *App) createMessageForm(w http.ResponseWriter, r *http.Request) {
	m, err := validate(r.FormValue("author"), r.FormValue("content"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.store.Add(m); err != nil {
		log.Println("error guardando mensaje:", err)
		http.Error(w, "no se pudo guardar el mensaje", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// GET /healthz -> 200 si la app puede escribir en su volumen, 503 si no.
func (a *App) healthz(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Writable(); err != nil {
		log.Println("healthcheck fallido:", err)
		http.Error(w, "unhealthy: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok\n"))
}
