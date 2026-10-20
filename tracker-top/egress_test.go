package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestEgressRotatesOnBan(t *testing.T) {
	// источник отдаёт 429 при обращении с первого «IP» (счётчик каналов:
	// первый запрос — прямой, помечается баном), второй — 200
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// два «канала» ротора: оба direct (для юнита важно поведение
	// выбора/остывания, а не сам прокси)
	egressChannels = []*egressChannel{
		{name: "a", client: srv.Client(), cooldown: time.Minute},
		{name: "b", client: srv.Client(), cooldown: time.Minute},
	}
	egressRR = 0

	req, _ := http.NewRequest("GET", srv.URL, nil)

	// первый запрос: канал a ловит 429 — ошибка и бан
	if _, err := egressDo(req); err == nil {
		t.Fatal("429 должен вернуть ошибку")
	}

	egressMu.Lock()
	banned := egressChannels[0].bannedUntil.After(time.Now())
	egressMu.Unlock()
	if !banned {
		t.Fatal("канал a должен уйти в остывание после 429")
	}

	// второй запрос уходит через живой канал — 200
	resp, err := egressDo(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("второй канал должен работать: %v", err)
	}
	resp.Body.Close()
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
