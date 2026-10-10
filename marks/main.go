// marks — «одна база» отметок Lampa: ручные отметки, история, «Просмотрено».
// Локальный кеш устройств и облако CUB нестабильны (viewed не синкается
// вовсе, history капит 500) — эта база хранит всё и восстанавливает
// устройства при любом старте с нуля.
//
//	GET  /m/<secret>/     — полный дамп: [{list, card, updated, deleted}]
//	POST /m/<secret>/     — слияние пачки (LWW по updated, tombstone = deleted)
//	GET  /healthz         — живость
//
// Хранилище — JSON-файл в дата-каталоге (атомарная замена через rename),
// объёмы личные (тысячи записей), SQL не окупается.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Mark — одна отметка; Card хранится целиком (title нужен для
// восстановления списков на чистом устройстве и матчинга по имени)
type Mark struct {
	List    string          `json:"list"`    // viewed | history | look | thrown | book | like | wath
	CardID  string          `json:"card_id"` // "<тип>:<tmdb id>"
	Card    json.RawMessage `json:"card"`
	Updated int64           `json:"updated"` // unix ms — LWW
	Deleted bool            `json:"deleted,omitempty"`
}

type store struct {
	mu    sync.Mutex
	path  string
	Marks map[string]Mark // ключ list + "|" + card_id
}

func load(path string) (*store, error) {
	s := &store{path: path, Marks: map[string]Mark{}}

	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.Marks == nil {
		s.Marks = map[string]Mark{}
	}
	return s, nil
}

// save — под атомарной заменой: пишем tmp, переименовываем
func (s *store) save() error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func key(list, cardID string) string { return list + "|" + cardID }

// merge — LWW: входящая запись побеждает при updated строго больше;
// tombstone (deleted) хранится, чтобы «снять отметку» разъезжался тоже
func (s *store) merge(batch []Mark) (applied int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range batch {
		if m.List == "" || m.CardID == "" || m.Updated == 0 {
			continue
		}
		k := key(m.List, m.CardID)
		if cur, ok := s.Marks[k]; !ok || m.Updated > cur.Updated {
			s.Marks[k] = m
			applied++
		}
	}
	return applied
}

func (s *store) all() []Mark {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Mark, 0, len(s.Marks))
	for _, m := range s.Marks {
		out = append(out, m)
	}
	return out
}

// buildMux — роуты сервиса (отдельно от main для тестов)
func buildMux(s *store, secret string) *http.ServeMux {
	mux := http.NewServeMux()
	prefix := "/m/" + secret + "/"

	mux.HandleFunc("GET "+prefix, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		enc := json.NewEncoder(w)
		_ = enc.Encode(s.all())
	})

	mux.HandleFunc("POST "+prefix, func(w http.ResponseWriter, r *http.Request) {
		// браузерный клиент шлёт text/plain (simple request, без префлайта),
		// тело — обычный JSON
		w.Header().Set("Access-Control-Allow-Origin", "*")

		var batch []Mark
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		applied := s.merge(batch)
		if err := s.save(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"applied":` + strconv(applied) + `}`))
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	return mux
}

func strconv(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func main() {
	addr := flag.String("addr", ":8356", "listen address")
	data := flag.String("data", "/data", "data directory")
	secret := flag.String("secret", "", "URL secret path segment (required)")
	flag.Parse()

	if *secret == "" {
		log.Fatal("marks: -secret is required")
	}
	if err := os.MkdirAll(*data, 0o700); err != nil {
		log.Fatal(err)
	}
	s, err := load(filepath.Join(*data, "marks.json"))
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           buildMux(s, *secret),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("marks: listening on %s (%d records)", *addr, len(s.all()))
	log.Fatal(srv.ListenAndServe())
}
