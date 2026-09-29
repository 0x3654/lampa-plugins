package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// jac.red — публичный Jackett-прокси cub (тот же парсер, что настраивается
// в Lampa). Один запрос = 8+ трекеров (nnmclub, rutor, rutracker, kinozal,
// megapeer, bitru, torrentby, korsars), поля стандартизированы.

const jackettBase = "https://jac.red"
const jackettKey = "1"

var jackettClient = &http.Client{Timeout: 25 * time.Second}

// дробилка на 429: публичный прокси банит пачками — после одного 429
// минуту не ходим (найдёт фолбэк/кэш), вместо десятков заведомо мёртвых
// запросов при сборке пулов
var (
	jackett429Mu sync.Mutex
	jackett429To time.Time
)

func jackettCooldown() bool {
	jackett429Mu.Lock()
	defer jackett429Mu.Unlock()
	return time.Now().Before(jackett429To)
}

func jackettBanned() {
	jackett429Mu.Lock()
	jackett429To = time.Now().Add(time.Minute)
	jackett429Mu.Unlock()
}

// год и русское название из Title Jackett
var (
	jTitleYearRe = regexp.MustCompile(`\((\d{4})`)
	jBaseCutRe   = regexp.MustCompile(`[(\[]`)
)

// jackettFind — кандидаты по названию (рус + orig), нормализованные в Item
func jackettFind(query, orig string) []Item {
	var out []Item
	seen := map[string]bool{}

	for _, q := range []string{query, orig} {
		if q == "" || seen["q:"+q] {
			continue
		}
		seen["q:"+q] = true
		if jackettCooldown() {
			return out
		}

		u := fmt.Sprintf("%s/api/v2.0/indexers/all/results?apikey=%s&query=%s",
			jackettBase, jackettKey, url.QueryEscape(q))

		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := jackettClient.Do(req)
		if err != nil {
			log.Printf("jackett: %v", err)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			jackettBanned()
			log.Printf("jackett: 429 — минутный отбой")
			return out
		}
		body := json.NewDecoder(resp.Body)
		var data struct {
			Results []struct {
				Title       string `json:"Title"`
				Tracker     string `json:"Tracker"`
				Seeders     int    `json:"Seeders"`
				Peers       int    `json:"Peers"`
				Size        int64  `json:"Size"`
				PublishDate string `json:"PublishDate"`
				Guid        string `json:"Guid"`
				Link        string `json:"Link"`
				Category    []int  `json:"Category"`
			} `json:"Results"`
		}
		err = body.Decode(&data)
		resp.Body.Close()
		if err != nil {
			continue
		}

		for _, r := range data.Results {
			title := r.Title

			ru, orig2, year, season := parseTitle(title)
			_ = orig2

			out = append(out, Item{
				Title:    title,
				Ru:       ru,
				Orig:     orig2,
				Year:     year,
				Season:   season,
				Seeders:  r.Seeders,
				Leechers: r.Peers,
				Size:     r.Size,
				Source:   "jackett:" + firstTracker(r.Tracker),
				URL:      r.Guid,
				Download: r.Link,
				Magnet:   magnetOf(r.Guid, r.Link),
				Quality:  parseQuality(title),
				Dub:      dubRe.MatchString(title),
				TsSound:  tsRe.MatchString(title),
				Cam:      camRe.MatchString(title),
				Mvo:      mvoRe.MatchString(title),
				SubOnly:  subOnlyOf(title),
				Voice:    voiceOf(title),
				Anime:    animeOfTracker(r.Tracker),
			})
		}
	}

	return out
}

func firstTracker(s string) string {
	if i := strings.IndexByte(s, ','); i > 0 {
		return s[:i]
	}
	return s
}

func magnetOf(guid, link string) string {
	if strings.HasPrefix(guid, "magnet:") {
		return guid
	}
	if strings.HasPrefix(link, "magnet:") {
		return link
	}
	return ""
}

func subOnlyOf(title string) bool {
	return (rutorSubRe.MatchString(title) || nnmSubRe.MatchString(title)) &&
		!dubRe.MatchString(title) && !mvoRe.MatchString(title) &&
		!rutorDubRe.MatchString(title) && !rutorMvoRe.MatchString(title)
}

func animeOfTracker(tracker string) bool {
	return strings.Contains(tracker, "anime") || strings.Contains(strings.ToLower(tracker), "anidub")
}

// jAtoi — Atoi без ошибки
func jAtoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
