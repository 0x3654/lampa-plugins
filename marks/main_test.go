package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func newTestServer(t *testing.T) (*store, *httptest.Server) {
	t.Helper()
	s, err := load(filepath.Join(t.TempDir(), "marks.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s, httptest.NewServer(buildMux(s, "test-secret"))
}

func TestMergeLWWAndTombstone(t *testing.T) {
	s, _ := newTestServer(t)

	card := json.RawMessage(`{"id":969681,"title":"Человек-паук"}`)

	// первая запись
	if got := s.merge([]Mark{{List: "viewed", CardID: "movie:969681", Card: card, Updated: 1000}}); got != 1 {
		t.Fatalf("первая запись: applied=%d, хочу 1", got)
	}
	// старая не перебивает новую
	if got := s.merge([]Mark{{List: "viewed", CardID: "movie:969681", Updated: 500}}); got != 0 {
		t.Fatalf("старая запись: applied=%d, хочу 0", got)
	}
	// tombstone новее — побеждает
	if got := s.merge([]Mark{{List: "viewed", CardID: "movie:969681", Updated: 2000, Deleted: true}}); got != 1 {
		t.Fatalf("tombstone: applied=%d, хочу 1", got)
	}
	if m := s.all(); len(m) != 1 || !m[0].Deleted {
		t.Fatalf("после tombstone: %+v", m)
	}
	// битые записи (без списка/ид/времени) молча пропускаются
	if got := s.merge([]Mark{{List: "", CardID: "x", Updated: 1}, {List: "viewed", Updated: 1}}); got != 0 {
		t.Fatalf("битые: applied=%d, хочу 0", got)
	}
}

func TestHTTPRoundtripAndAuth(t *testing.T) {
	_, srv := newTestServer(t)
	defer srv.Close()

	// без секрета — 404
	if resp, err := http.Get(srv.URL + "/m/wrong/"); err != nil || resp.StatusCode != 404 {
		t.Fatalf("чужой секрет: resp=%v err=%v", resp, err)
	}

	// пишем
	body, _ := json.Marshal([]Mark{{List: "history", CardID: "movie:123", Card: json.RawMessage(`{"id":123}`), Updated: 1}})
	resp, err := http.Post(srv.URL+"/m/test-secret/", "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("post: %v %v", resp, err)
	}

	// читаем
	resp, err = http.Get(srv.URL + "/m/test-secret/")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("get: %v %v", resp, err)
	}
	var all []Mark
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].List != "history" {
		t.Fatalf("дамп: %+v", all)
	}
}

func TestPersistenceAcrossReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "marks.json")

	s1, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	s1.merge([]Mark{{List: "viewed", CardID: "movie:9", Updated: 5}})
	if err := s1.save(); err != nil {
		t.Fatal(err)
	}

	s2, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.all()) != 1 {
		t.Fatalf("после перезагрузки: %+v", s2.all())
	}
}
