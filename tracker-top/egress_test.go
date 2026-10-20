package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestEgressRotatesOnBan(t *testing.T) {
	// источник отдаёт 403 на первый запрос (IP-бан), 200 — на второй;
	// канал a ловит бан, канал b продолжает работать
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(403)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	egressChannels = []*egressChannel{
		{name: "a", client: srv.Client(), cooldown: time.Minute},
		{name: "b", client: srv.Client(), cooldown: time.Minute},
	}
	egressRR = 0

	req, _ := http.NewRequest("GET", srv.URL, nil)

	if _, err := egressDo(req); err == nil {
		t.Fatal("403 должен вернуть ошибку")
	}

	egressMu.Lock()
	banned := egressChannels[0].bannedUntil.After(time.Now())
	egressMu.Unlock()
	if !banned {
		t.Fatal("канал a должен уйти в остывание после 403")
	}

	resp, err := egressDo(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("второй канал должен работать: %v", err)
	}
	resp.Body.Close()
}

func TestEgress429DoesNotBanChannel(t *testing.T) {
	// 429 — лимит по ключу (Jackett), не по IP: канал не остывает,
	// ответ возвращается вызывающему для глобальной обработки
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer srv.Close()

	egressChannels = []*egressChannel{
		{name: "only", client: srv.Client(), cooldown: time.Minute},
	}
	egressRR = 0

	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := egressDo(req)
	if err != nil {
		t.Fatalf("429 возвращается без ошибки ротора: %v", err)
	}
	resp.Body.Close()

	egressMu.Lock()
	banned := egressChannels[0].bannedUntil.After(time.Now())
	egressMu.Unlock()
	if banned {
		t.Fatal("429 не должен банить канал")
	}
}

func TestEgressCooldownGrows(t *testing.T) {
	c := &egressChannel{name: "x", cooldown: 5 * time.Minute}
	egressChannels = []*egressChannel{c}

	c.ban("первый")
	first := c.cooldown
	if first != 15*time.Minute {
		t.Fatalf("кулдаун 5м → 15м, получили %s", first)
	}
	c.ban("второй")
	if c.cooldown != 45*time.Minute {
		t.Fatalf("кулдаун 15м → 45м, получили %s", c.cooldown)
	}
	c.ban("третий")
	c.ban("четвёртый")
	if c.cooldown != 2*time.Hour {
		t.Fatalf("потолок 2ч, получили %s", c.cooldown)
	}
}

func TestEgressInitParsesSpec(t *testing.T) {
	t.Setenv("ROTOR_PROXY", "direct, ae=socks5://172.17.0.1:3131")
	egressChannels = nil
	egressInit()

	if len(egressChannels) != 2 {
		t.Fatalf("2 канала, получили %d", len(egressChannels))
	}
	if egressChannels[0].name != "direct" || egressChannels[1].name != "ae" {
		t.Fatalf("имена каналов: %s, %s", egressChannels[0].name, egressChannels[1].name)
	}
	if egressChannels[1].client.Transport == nil {
		t.Fatal("у прокси-канала должен быть транспорт с прокси")
	}
}
