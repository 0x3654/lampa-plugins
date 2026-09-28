package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// «Топ · TMDB» целиком на сервере: список + фильтры раздач + качество.
// Lampa остаётся только фильтр по просмотрам.

const tmdbKey = "4ef0d7355d9ffb5151e987764708ce96" // публичный ключ клиента Lampa
const tmdbAPI = "https://api.themoviedb.org/3/"

// варианты — те же, что в плагине
type tmdbVariant struct {
	Method     string
	WindowDays int
	DateKey    string
	Params     map[string]string
}

var tmdbVariants = map[string]tmdbVariant{
	"movie_week": {Method: "trending/movie/week"},
	"movie_day":  {Method: "trending/movie/day"},
	"movie_14":   {Method: "discover/movie", WindowDays: 14, DateKey: "primary_release_date", Params: map[string]string{"sort_by": "popularity.desc", "vote_count.gte": "50"}},
	"movie_30":   {Method: "discover/movie", WindowDays: 30, DateKey: "primary_release_date", Params: map[string]string{"sort_by": "popularity.desc", "vote_count.gte": "50"}},
	"tv_week":    {Method: "trending/tv/week"},
	"tv_day":     {Method: "trending/tv/day"},
	"tv_30":      {Method: "discover/tv", WindowDays: 30, DateKey: "first_air_date", Params: map[string]string{"sort_by": "popularity.desc", "vote_count.gte": "20"}},
	"movie_best": {Method: "discover/movie", Params: map[string]string{"sort_by": "vote_average.desc", "vote_count.gte": "2000"}},
	"tv_best":    {Method: "discover/tv", Params: map[string]string{"sort_by": "vote_average.desc", "vote_count.gte": "1500"}},
	"movie_2025": {Method: "discover/movie", Params: map[string]string{"sort_by": "popularity.desc", "primary_release_date.gte": "2025-01-01", "vote_count.gte": "100"}},
	"tv_2025":    {Method: "discover/tv", Params: map[string]string{"sort_by": "popularity.desc", "first_air_date.gte": "2025-01-01", "vote_count.gte": "30"}},
}

// кэш TMDB-страниц: список карточек по variant+page (тренды обновляются редко)
var (
	tmdbCacheMu sync.Mutex
	tmdbCache   = map[string]tmdbCacheEntry{}
)

type tmdbCacheEntry struct {
	ts      time.Time
	results []map[string]any
}

func fetchTMDB(v tmdbVariant, page int) ([]map[string]any, error) {
	key := fmt.Sprintf("%s|%d", v.Method, page)

	tmdbCacheMu.Lock()
	if e, ok := tmdbCache[key]; ok && time.Since(e.ts) < 10*time.Minute {
		tmdbCacheMu.Unlock()
		return e.results, nil
	}
	tmdbCacheMu.Unlock()

	q := url.Values{}
	q.Set("api_key", tmdbKey)
	q.Set("language", "ru")
	q.Set("page", strconv.Itoa(page))
	for k, val := range v.Params {
		q.Set(k, val)
	}
	if v.WindowDays > 0 && v.DateKey != "" {
		q.Set(v.DateKey+".gte", time.Now().AddDate(0, 0, -v.WindowDays).Format("2006-01-02"))
	}

	req, err := http.NewRequest("GET", tmdbAPI+v.Method+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tmdb: HTTP %d", resp.StatusCode)
	}

	var body struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	tmdbCacheMu.Lock()
	tmdbCache[key] = tmdbCacheEntry{ts: time.Now(), results: body.Results}
	if len(tmdbCache) > 200 {
		tmdbCache = map[string]tmdbCacheEntry{}
	}
	tmdbCacheMu.Unlock()

	return body.Results, nil
}

// кэш результатов /feed (ключ включает фильтры и exclude юзера):
// повторные запросы той же страницы — мгновенно
var (
	feedResMu sync.Mutex
	feedRes   = map[string]feedResEntry{}
)

type feedResEntry struct {
	ts      time.Time
	results []map[string]any
	total   int
}

// buildFeed: страница из `pageSize` карточек, прошедших фильтры раздач —
// сервер добирает TMDB-страницы, пока не наберёт (фильтрация больше
// не съедает карточки, как было на клиенте)
func buildFeedCached(variant string, page, feedSize int, minq, voices string, junk, ru bool, exclude map[int]bool) ([]map[string]any, int, error) {
	// ключ: вариант+страница+фильтры+отсортированный exclude
	ex := make([]string, 0, len(exclude))
	for id := range exclude {
		ex = append(ex, strconv.Itoa(id))
	}
	sort.Strings(ex)
	key := fmt.Sprintf("%s|%d|%d|%s|%s|%v|%v|%s", variant, page, feedSize, minq, voices, junk, ru, strings.Join(ex, ","))

	feedResMu.Lock()
	if e, ok := feedRes[key]; ok && time.Since(e.ts) < 5*time.Minute {
		feedResMu.Unlock()
		return e.results, e.total, nil
	}
	feedResMu.Unlock()

	results, total, err := buildFeed(variant, page, feedSize, minq, voices, junk, ru, exclude)
	if err != nil {
		return nil, 0, err
	}

	feedResMu.Lock()
	if len(feedRes) > 100 {
		feedRes = map[string]feedResEntry{}
	}
	feedRes[key] = feedResEntry{ts: time.Now(), results: results, total: total}
	feedResMu.Unlock()

	return results, total, nil
}

func buildFeed(variant string, page, feedSize int, minq, voices string, junk, ru bool, exclude map[int]bool) ([]map[string]any, int, error) {
	v, ok := tmdbVariants[variant]
	if !ok {
		return nil, 0, fmt.Errorf("unknown variant")
	}

	filters := minq != "" && minq != "any"

	// карта вариантов без фильтров: страница = срез кэша TMDB
	if !filters && !junk && ru {
		// всё равно фильтруем найденное: только с раздачами — фильтр всегда включён
	}

	var collected []map[string]any
	seen := map[float64]bool{} // TMDB id — тренды дублируют фильмы между страницами
	var skipped int
	need := feedSize * page
	tmdbPage := 1

	for len(collected) < need && tmdbPage <= 40 {
		results, err := fetchTMDB(v, tmdbPage)
		if err != nil {
			return nil, 0, err
		}
		if len(results) == 0 {
			break
		}

		for _, el := range results {
			if isAnimePerson(el) {
				continue
			}

			// просмотренные: клиент прислал свои id — страница считается
			// ПОСЛЕ фильтра, потому всегда полная
			if idf, ok := el["id"].(float64); ok && exclude != nil && exclude[int(idf)] {
				continue
			}
			query, _ := el["title"].(string)
			if query == "" {
				query, _ = el["name"].(string)
			}
			orig, _ := el["original_title"].(string)
			if orig == "" {
				orig, _ = el["original_name"].(string)
			}
			date, _ := el["release_date"].(string)
			if date == "" {
				date, _ = el["first_air_date"].(string)
			}
			year, _ := strconv.Atoi(strings.TrimSpace(date[:min(4, len(date))]))
			typ := "movie"
			if _, has := el["name"]; has {
				typ = "tv"
			}

			it, found := findWithCache2(query, orig, year, typ, minq, voices, junk, ru)
			if !found {
				skipped++
				continue
			}

			el["quality"] = humanQuality(it.Quality)
			if v := humanVoice(it); v != "" {
				el["voice"] = v
			}

			if id, ok := el["id"].(float64); ok {
				if seen[id] {
					continue // дубль с соседней TMDB-страницы
				}
				seen[id] = true
			}
			collected = append(collected, el)
		}

		tmdbPage++
	}

	// вырезаем просмотренные на клиенте; total_pages по собранному
	total := (len(collected) + feedSize - 1) / feedSize
	lo := (page - 1) * feedSize
	hi := lo + feedSize
	if lo > len(collected) {
		lo = len(collected)
	}
	if hi > len(collected) {
		hi = len(collected)
	}

	_ = skipped
	return collected[lo:hi], total, nil
}

func isAnimePerson(el map[string]any) bool {
	// медиа-тип person в trendingAll не нужен
	if mt, _ := el["media_type"].(string); mt == "person" {
		return true
	}
	return false
}

// humanVoice — озвучка лучшей раздачи: приоритет Дубляж > Многоголосый > студия
func humanVoice(it Item) string {
	switch {
	case it.Dub:
		return "Дубляж"
	case it.Mvo:
		return "Многоголосый"
	case it.Voice != "":
		return it.Voice
	}
	return ""
}

// humanQuality — сырой ранг → вид для плашки
func humanQuality(q string) string {
	switch q {
	case "2160":
		return "4K"
	case "1080":
		return "1080p"
	case "720":
		return "720p"
	}
	return "SD"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = utf8.ValidString

// ---------- пул вариантов: 100 фильмов на вариант, обновление 2 раза в день
// Любая страница с любым exclude юзера нарезается из горячего пула мгновенно.

var (
	poolMu sync.Mutex
	pools  = map[string]poolEntry{}
)

type poolEntry struct {
	ts      time.Time
	results []map[string]any
}

const poolSize = 500 // глубокий пул: 100 на клиента после любых фильтров
const poolTTL = 12 * time.Hour

// getPool — пул варианта (собирает при промахе; вызывается прогревом и /feed)
func getPool(variant string) ([]map[string]any, error) {
	poolMu.Lock()
	if e, ok := pools[variant]; ok && time.Since(e.ts) < poolTTL {
		poolMu.Unlock()
		return e.results, nil
	}
	poolMu.Unlock()

	// сборка: те же фильтры раздач, что и всегда (junk/ru, без озвучек юзера —
	// озвучечные фильтры узкие, их оставим на поиске, пул — базовый)
	v, ok := tmdbVariants[variant]
	if !ok {
		return nil, fmt.Errorf("unknown variant")
	}

	var collected []map[string]any
	seen := map[float64]bool{}
	tmdbPage := 1

	for len(collected) < poolSize && tmdbPage <= 20 {
		results, err := fetchTMDB(v, tmdbPage)
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			break
		}

		for _, el := range results {
			if isAnimePerson(el) {
				continue
			}
			if idf, ok := el["id"].(float64); ok && seen[idf] {
				continue
			}

			query, _ := el["title"].(string)
			if query == "" {
				query, _ = el["name"].(string)
			}
			orig, _ := el["original_title"].(string)
			if orig == "" {
				orig, _ = el["original_name"].(string)
			}
			date, _ := el["release_date"].(string)
			if date == "" {
				date, _ = el["first_air_date"].(string)
			}
			year := 0
			if len(date) >= 4 {
				year, _ = strconv.Atoi(date[:4])
			}
			typ := "movie"
			if _, has := el["name"]; has {
				typ = "tv"
			}

			it, found := findWithCache2(query, orig, year, typ, "", "", true, true)
			if !found {
				continue
			}

			el["quality"] = humanQuality(it.Quality)
			if vv := humanVoice(it); vv != "" {
				el["voice"] = vv
			}
			if idf, ok := el["id"].(float64); ok {
				seen[idf] = true
			}
			collected = append(collected, el)
			if len(collected) >= poolSize {
				break
			}
		}

		tmdbPage++
	}

	poolMu.Lock()
	pools[variant] = poolEntry{ts: time.Now(), results: collected}
	if len(pools) > 30 {
		pools = map[string]poolEntry{}
	}
	poolMu.Unlock()

	return collected, nil
}

// poolPage — страница из пула с учётом exclude юзера
func poolPage(variant string, page, feedSize int, exclude map[int]bool) ([]map[string]any, int, error) {
	all, err := getPool(variant)
	if err != nil {
		return nil, 0, err
	}

	filtered := make([]map[string]any, 0, len(all))
	for _, el := range all {
		if idf, ok := el["id"].(float64); ok && exclude != nil && exclude[int(idf)] {
			continue
		}
		filtered = append(filtered, el)
	}

	total := (len(filtered) + feedSize - 1) / feedSize
	lo := (page - 1) * feedSize
	hi := lo + feedSize
	if lo > len(filtered) {
		lo = len(filtered)
	}
	if hi > len(filtered) {
		hi = len(filtered)
	}

	return filtered[lo:hi], total, nil
}
