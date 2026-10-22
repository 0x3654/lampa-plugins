package main

// jacred — источник раздач из своего агрегатора JacRed (Torznab API,
// инстансы на ae2 и micro дополняют базу синком). Заменяет прямые заходы
// в трекеры и Jackett-прокси: парсер сам ходит по трекерам, обходит
// Cloudflare и уже отдаёт magnet/сидов/год атрибутами.

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	jacredURL   = env("JACRED_URL", "http://127.0.0.1:9117/api/v1/indexer/1/newznab")
	jacredKey   = env("JACRED_KEY", "")
	jacredProxy = env("JACRED_PROXY", "socks5://172.17.0.1:3132") // канал ae2 в egress-роторе
)

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

