package main

// jacred — источник раздач из своего агрегатора JacRed (Torznab API,
// инстансы на ae2 и micro дополняют базу синком). Заменяет прямые заходы
// в трекеры и Jackett-прокси: парсер сам ходит по трекерам, обходит
// Cloudflare и уже отдаёт magnet/сидов/год атрибутами.

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	jacredURL = env("JACRED_URL", "http://127.0.0.1:9117/api/v1/indexer/1/newznab")
	jacredKey = env("JACRED_KEY", "")
	// канал ae2 в egress-роторе; ЯВНО пустая переменная (JACRED_PROXY=) —
	// без прокси: инстансы рядом с агрегатором ходят напрямую
	jacredProxy = "socks5://172.17.0.1:3132"
)

func init() {
	if v, ok := os.LookupEnv("JACRED_PROXY"); ok {
		jacredProxy = v
	}
}

var jacredClient *http.Client

func jacredInit() {
	if jacredKey == "" {
		return // без ключа источник отключен — фолбэк на старый путь
	}

	tr := &http.Transport{
		MaxIdleConns:        4,
		IdleConnTimeout:     60 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if jacredProxy != "" {
		if u, err := url.Parse(jacredProxy); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}

	jacredClient = &http.Client{Timeout: 20 * time.Second, Transport: tr}
}

// torznabFeed —/rss newznab: item'ы с torznab:attr
type torznabFeed struct {
	Items []struct {
		Title string `xml:"title"`
		Attrs []struct {
			Name  string `xml:"name,attr"`
			Value string `xml:"value,attr"`
		} `xml:"attr"`
	} `xml:"channel>item"`
}

// jacredItemFromTitle — title + torznab-атрибуты → Item: ru/orig/год/
// качество/озвучки вытаскиваем из названия нашим парсером (title несёт
// всё: «Глубина / Below / Сезон: 1 … WEB-DL 1080p» Dub), magnet/сидеры/
// размер/источник — готовыми атрибутами
func jacredItemFromTitle(title string, attrs []struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}) Item {
	ru, orig, year, _ := parseTitle(title)

	item := Item{
		Title:   title,
		Ru:      ru,
		Orig:    orig,
		Year:    year,
		Quality: parseQuality(title),
		Dub:     dubRe.MatchString(title),
		TsSound: tsRe.MatchString(title),
		Cam:     camRe.MatchString(title),
		Mvo:     mvoRe.MatchString(title),
		SubOnly: subOnlyOf(title),
		Voice:   voiceOf(title),
		Source:  "jacred",
	}

	for _, a := range attrs {
		switch a.Name {
		case "magneturl":
			item.Magnet = a.Value
		case "seeders":
			item.Seeders = atoi(a.Value)
		case "leechers":
			item.Leechers = atoi(a.Value)
		case "size":
			item.Size = atoi64(a.Value)
		case "site":
			item.Source = "jacred:" + a.Value
		case "year":
			if item.Year == 0 {
				item.Year = atoi(a.Value)
			}
		}
	}

	return item
}

// jacredFind — кандидаты по названию (рус + orig), нормализованные в Item.
// Год/качество/озвучки — из названия нашим парсером (title несёт всё:
// «Глубина / Below / Сезон: 1 … WEB-DL 1080p» Dub), magnet/сидеры —
// готовыми атрибутами.
func jacredFind(query, orig string) []Item {
	if jacredClient == nil {
		return nil
	}

	var out []Item
	seenQ := map[string]bool{}

	for _, q := range []string{query, orig} {
		if q == "" || seenQ["q:"+q] {
			continue
		}
		seenQ["q:"+q] = true

		u := fmt.Sprintf("%s?t=search&q=%s&apikey=%s&limit=100",
			jacredURL, url.QueryEscape(q), url.QueryEscape(jacredKey))

		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := jacredClient.Do(req)
		if err != nil {
			log.Printf("jacred: %v", err)
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			log.Printf("jacred: %s: HTTP %d", q, resp.StatusCode)
			continue
		}

		var feed torznabFeed
		err = xml.NewDecoder(resp.Body).Decode(&feed)
		resp.Body.Close()
		if err != nil {
			log.Printf("jacred: разбор: %v", err)
			continue
		}

		if len(feed.Items) == 0 {
			log.Printf("jacred: «%s» — пусто", q)
		} else {
			log.Printf("jacred: «%s» — %d item", q, len(feed.Items))
		}

		for _, it := range feed.Items {
			if it.Title == "" {
				continue
			}

			out = append(out, jacredItemFromTitle(it.Title, it.Attrs))
		}
	}

	return out
}

func atoi(s string) int {
	n := 0
	for _, c := range strings.TrimSpace(s) {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
		if n > 1<<30 {
			return n
		}
	}
	return n
}

func atoi64(s string) int64 {
	var n int64
	for _, c := range strings.TrimSpace(s) {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int64(c-'0')
		if n > 1<<62 {
			return n
		}
	}
	return n
}

// ---------- нативный топ агрегатора: /api/v1.0/top (свой эндпоинт форка)

// topTime — DateTime агрегатора приходит без зоны (Newtonsoft:
// «2026-10-08T10:11:00»), Go ждёт RFC3339; парсим несколькими layout
type topTime time.Time

func (t *topTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*t = topTime(time.Time{})
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.9999999", "2006-01-02T15:04:05"} {
		if v, err := time.Parse(layout, s); err == nil {
			*t = topTime(v)
			return nil
		}
	}
	*t = topTime(time.Time{}) // битое время не роняет строку топа
	return nil
}

// topRow — строка топа: агрегат всех раздач тайтла (шард masterDb),
// лучшая раздача по сидам задаёт имя/год/качество/озвучку
type topBest struct {
	Tracker    string   `json:"tracker"`
	URL        string   `json:"url"`
	Magnet     string   `json:"magnet"`
	Title      string   `json:"title"`
	Sid        int      `json:"sid"`
	Size       float64  `json:"size"`
	SizeName   string   `json:"sizeName"`
	Quality    int      `json:"quality"`
	Videotype  string   `json:"videotype"`
	Voices     []string `json:"voices"`
	Seasons    []int    `json:"seasons"`
	CreateTime topTime  `json:"createTime"`
}

type topRow struct {
	Key          string   `json:"key"`
	Name         string   `json:"name"`
	Originalname string   `json:"originalname"`
	Relased      int      `json:"relased"`
	Type         string   `json:"type"`
	MaxSid       int      `json:"maxSid"`
	SumSid       int      `json:"sumSid"`
	Releases     int      `json:"releases"`
	Trackers     []string `json:"trackers"`
	LastUpdate   topTime  `json:"lastUpdate"`
	Best         *topBest `json:"best"`
}

type topResp struct {
	UpdatedAt topTime  `json:"updatedAt"`
	Count     int      `json:"count"`
	Rows      []topRow `json:"rows"`
}

// qualityFromInt — quality у JacRed в пикселях высоты кадра (2160/1080/720…)
func qualityFromInt(q int) string {
	switch {
	case q >= 2160:
		return "2160"
	case q >= 1080:
		return "1080"
	case q >= 720:
		return "720"
	}
	return "sd"
}

// jacredBase — корень API из JACRED_URL (тот указывает на torznab-путь)
func jacredBase() string {
	if u, err := url.Parse(jacredURL); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return ""
}

// jacredTop — «Популярное»: топ агрегатора по всем трекерам.
// sort: "sid" (по сидам) | "create" (новинки)
func jacredTop(sort string) []Item {
	if jacredClient == nil {
		return nil
	}

	base := jacredBase()
	if base == "" {
		return nil
	}

	u := base + "/api/v1.0/top?take=1000"
	if sort != "" {
		u += "&sort=" + url.QueryEscape(sort)
	}

	// свой дедлайн: пересборка на стороне агрегатора может держать запрос
	// дольше общего 20с-таймаута клиента поиска
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("X-Api-Key", jacredKey)

	resp, err := jacredClient.Do(req)
	if err != nil {
		log.Printf("jacred-top: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("jacred-top: HTTP %d", resp.StatusCode)
		return nil
	}

	var top topResp
	if err := json.NewDecoder(resp.Body).Decode(&top); err != nil {
		log.Printf("jacred-top: разбор: %v", err)
		return nil
	}

	return topRowsToItems(top.Rows)
}

// topRowsToItems — строки топа → Item: флаги озвучки из канонических
// voices агрегатора («Дубляж») плюс наши регулярки по названию раздачи
func topRowsToItems(rows []topRow) []Item {
	items := make([]Item, 0, len(rows))

	for _, r := range rows {
		if r.Best == nil || r.Name == "" {
			continue
		}
		b := r.Best

		it := Item{
			Ru:       r.Name,
			Orig:     r.Originalname,
			Year:     r.Relased,
			Season:   r.Type == "tv" || len(b.Seasons) > 0,
			Title:    b.Title,
			Seeders:  b.Sid,
			Size:     int64(b.Size),
			SizeText: b.SizeName,
			URL:      b.URL,
			Magnet:   b.Magnet,
			Source:   "jacred:" + b.Tracker,
			Quality:  qualityFromInt(b.Quality),
			Dub:      dubRe.MatchString(b.Title),
			TsSound:  tsRe.MatchString(b.Title),
			Cam:      camRe.MatchString(b.Title),
			Mvo:      mvoRe.MatchString(b.Title),
			SubOnly:  subOnlyOf(b.Title),
			Voice:    voiceOf(b.Title),
		}

		for _, v := range b.Voices {
			if v == "Дубляж" {
				it.Dub = true
			}
		}

		// названия без явной озвучки (regex пусто): бейджем станет студия
		if it.Voice == "" {
			for _, v := range b.Voices {
				if v != "" {
					it.Voice = v
					break
				}
			}
		}

		items = append(items, it)
	}

	return items
}
