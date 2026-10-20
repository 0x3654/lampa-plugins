package main

// egress — ротор исходящих каналов: прямые запросы и SOCKS5-туннели через
// свои VPS (ae/ae2/micro, SSH -D на docker-gateway). Трекеры и Jackett
// банят датацентровые IP точечно — канал, словивший бан/лимит, уходит
// в остывание (растущее), запрос идёт через следующий живой.

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type egressChannel struct {
	name        string
	client      *http.Client
	bannedUntil time.Time
	cooldown    time.Duration // растёт с каждым баном
}

var (
	egressMu       sync.Mutex
	egressChannels []*egressChannel
	egressRR       int
)

// egressInit — каналы из env ROTOR_PROXY:
//
//	direct,ae=socks5://172.17.0.1:3131,ae2=socks5://172.17.0.1:3132
//
// «direct» обязателен по смыслу, остальные — именованные прокси.
func egressInit() {
	spec := os.Getenv("ROTOR_PROXY")
	if spec == "" {
		spec = "direct"
	}

	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		name, addr := "direct", part
		if i := strings.IndexByte(part, '='); i > 0 {
			name, addr = part[:i], part[i+1:]
		}

		tr := &http.Transport{
			MaxIdleConns:        8,
			IdleConnTimeout:     60 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}

		if addr != "direct" {
			u, err := url.Parse(addr)
			if err != nil {
				continue
			}
			tr.Proxy = http.ProxyURL(u)
		}

		egressChannels = append(egressChannels, &egressChannel{
			name:    name,
			client:  &http.Client{Timeout: 25 * time.Second, Transport: tr},
			cooldown: 5 * time.Minute,
		})
	}

	if len(egressChannels) == 0 {
		egressChannels = []*egressChannel{{
			name:    "direct",
			client:  &http.Client{Timeout: 25 * time.Second},
			cooldown: 5 * time.Minute,
		}}
	}

	log.Printf("egress: %d каналов: %s", len(egressChannels), egressNames())
}

func egressNames() string {
	var b strings.Builder
	for i, c := range egressChannels {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(c.name)
	}
	return b.String()
}

// ban — канал в остывание; кулдаун растёт (5м → 15м → 45м, потолок 2ч)
func (c *egressChannel) ban(reason string) {
	egressMu.Lock()
	defer egressMu.Unlock()

	c.bannedUntil = time.Now().Add(c.cooldown)
	c.cooldown *= 3
	if c.cooldown > 2*time.Hour {
		c.cooldown = 2 * time.Hour
	}

	log.Printf("egress: канал %s остывает %s (%s)", c.name, c.cooldown, reason)
}

// egressPick — следующий живой канал (round-robin); все остыли — самый
// давно остывший (лучше попытка, чем отказ)
func egressPick() *egressChannel {
	egressMu.Lock()
	defer egressMu.Unlock()

	now := time.Now()

	var oldest *egressChannel
	for i := 0; i < len(egressChannels); i++ {
		c := egressChannels[(egressRR+i)%len(egressChannels)]
		if now.After(c.bannedUntil) {
			egressRR = (egressRR + i + 1) % len(egressChannels)
			return c
		}
		if oldest == nil || c.bannedUntil.Before(oldest.bannedUntil) {
			oldest = c
		}
	}

	return oldest
}

// egressDo — запрос через ротор. Канал остывает при 403 (IP-бан
// источника — следующий запрос выйдет с другого IP) и сетевых сбоях
// (туннель лёг). 429 канал НЕ банит: Jackett-прокси лимитирует по
// общему apikey куба, не по IP — глобальный минутный отбой обрабатывает
// вызывающий код, а ротор не должен замораживать канал для остальных
// источников.
func egressDo(req *http.Request) (*http.Response, error) {
	ch := egressPick()

	resp, err := ch.client.Do(req)
	if err != nil {
		ch.ban(err.Error())
		return nil, err
	}

	if resp.StatusCode == 403 {
		resp.Body.Close()
		ch.ban(fmt.Sprintf("HTTP 403 от %s", req.Host))
		return nil, fmt.Errorf("%s: HTTP 403 (канал %s)", req.URL.Host, ch.name)
	}

	return resp, nil
}

// egressStatus — живые/остывающие каналы для /healthz
func egressStatus() []map[string]any {
	egressMu.Lock()
	defer egressMu.Unlock()

	now := time.Now()
	out := make([]map[string]any, 0, len(egressChannels))
	for _, c := range egressChannels {
		out = append(out, map[string]any{
			"name":    c.name,
			"banned":  now.Before(c.bannedUntil),
			"unban_s": int(time.Until(c.bannedUntil).Seconds()),
		})
	}
	return out
}
