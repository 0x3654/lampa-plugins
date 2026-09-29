package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	// три смысла × фильмы/сериалы (тренды недели / лучшее / новинки 2025+);
	// день, окна 14/30 дней убраны: совпадали с трендами на 78–100%
	"movie_week": {Method: "trending/movie/week"},
	"movie_best": {Method: "discover/movie", Params: map[string]string{"sort_by": "vote_average.desc", "vote_count.gte": "2000"}},
	"movie_2025": {Method: "discover/movie", Params: map[string]string{"sort_by": "popularity.desc", "primary_release_date.gte": "2025-01-01", "vote_count.gte": "100"}},
	"tv_week":    {Method: "trending/tv/week"},
	"tv_best":    {Method: "discover/tv", Params: map[string]string{"sort_by": "vote_average.desc", "vote_count.gte": "1500"}},
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
	// ключ — весь профиль варианта, НЕ только метод: у discover/movie четыре
	// варианта (окна 14/30, лучшее, новинки) — по методу они склеивались
	// в один кэш и все получали вселенную первого собранного
	key := fmt.Sprintf("%s|%s|%d|%v|%d", v.Method, v.DateKey, v.WindowDays, v.Params, page)

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
	poolMu   sync.Mutex
	poolBusy = map[string]bool{}
	pools    = map[string]poolEntry{}
)

type poolEntry struct {
	ts      time.Time
	results []map[string]any
}

const poolSize = 500 // глубокий пул: 100 на клиента после любых фильтров
const poolTTL = 12 * time.Hour

// findInTopBase — матч по базе «Топ · трекеров» (прогрета warm-циклом):
// база знает ru-названия/год/качество/озвучку фактически, расхождений
// «в трекерах есть — find не нашёл» не остаётся
func findInTopBase(query, orig string, year int, typ string) (Item, bool) {
	payload, _, err := getTop("both", "video", 3, true, "", "1", "seeds", "0")
	if err != nil {
		return Item{}, false
	}

	nq, no := normName(query), normName(orig)
	tol := 1
	if typ == "tv" {
		tol = 6
	}

	var best *Item
	for i := range payload.Items {
		it := payload.Items[i]
		if it.Year == 0 || year == 0 || abs(it.Year-year) > tol {
			continue
		}
		ruM := normName(it.Ru) == nq && (no == "" || it.Orig == "" || normName(it.Orig) == no)
		oM := no != "" && normName(it.Orig) == no
		if !ruM && !oM {
			continue
		}
		if best == nil || betterRelease(it, *best) {
			cp := it
			best = &cp
		}
	}

	if best == nil {
		return Item{}, false
	}
	return *best, true
}

// errPoolBuilding — пул собирается (обычно прогревом): /feed отдаёт сырую
// страницу TMDB мгновенно, качество появится следующим открытием
var errPoolBuilding = errors.New("pool building")

// getPool — пул варианта (собирает при промахе; вызывается прогревом и /feed);
// пока сборка идёт, повторные вызовы не ждут и не дублируют работу
// getPool — пул варианта; сборка ВСЕГДА в фоне: первый запрос к варианту,
// до которого не дошёл прогрев, запускает сборку горутиной и сразу получает
// errPoolBuilding (клиенту — сырая страница TMDB), а не висит минуты в
// синхронной сборке (nginx рвёт связь по 60с — 504)
func getPool(variant string) ([]map[string]any, error) {
	poolMu.Lock()
	if e, ok := pools[variant]; ok && time.Since(e.ts) < poolTTL {
		poolMu.Unlock()
		return e.results, nil
	}
	if poolBusy[variant] {
		poolMu.Unlock()
		return nil, errPoolBuilding
	}
	poolBusy[variant] = true
	poolMu.Unlock()

	go func() {
		defer func() {
			poolMu.Lock()
			delete(poolBusy, variant)
			poolMu.Unlock()
		}()

		collected, err := buildPool(variant)
		if err != nil {
			log.Printf("pool %s: %v", variant, err)
			return
		}

		// тонкий пул (транзиентная ошибка TMDB-прокси на сборке) не живёт
		// 12ч: короткий TTL — следующее обращение пересоберёт
		ts := time.Now()
		if len(collected) < 50 {
			ts = ts.Add(-poolTTL + 5*time.Minute)
		}
		poolMu.Lock()
		pools[variant] = poolEntry{ts: ts, results: collected}
		if len(pools) > 30 {
			pools = map[string]poolEntry{}
		}
		poolMu.Unlock()
	}()

	return nil, errPoolBuilding
}

// buildPool — сборка: те же фильтры раздач, что и всегда (junk/ru, без
// озвучек юзера — озвучечные фильтры узкие, их оставим на поиске, пул — базовый)
func buildPool(variant string) ([]map[string]any, error) {

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

			it, found := findInTopBase(query, orig, year, typ)
			if !found {
				it, found = findWithCache2(query, orig, year, typ, "", "", true, true)
			}
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

	return collected, nil
}

// rawTMDBPage — сырая страница TMDB без проверки раздач (мгновенно):
// ответ /feed, пока пул собирается; карточки без качества/озвучки
func rawTMDBPage(variant string, page, feedSize int, exclude map[int]bool) ([]map[string]any, int, error) {
	v, ok := tmdbVariants[variant]
	if !ok {
		return nil, 0, fmt.Errorf("unknown variant")
	}

	var collected []map[string]any
	seen := map[float64]bool{}
	tmdbPage := 1

	for len(collected) < feedSize*page && tmdbPage <= 40 {
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
			if idf, ok := el["id"].(float64); ok {
				if seen[idf] || (exclude != nil && exclude[int(idf)]) {
					continue
				}
				seen[idf] = true
			}
			collected = append(collected, el)
		}
		tmdbPage++
	}

	total := (len(collected) + feedSize - 1) / feedSize
	lo := (page - 1) * feedSize
	hi := lo + feedSize
	if lo > len(collected) {
		lo = len(collected)
	}
	if hi > len(collected) {
		hi = len(collected)
	}
	return collected[lo:hi], total, nil
}

// cardQualityRank — ранг человеческого качества карточки (из humanQuality)
var cardQualityRank = map[string]int{"SD": 0, "720p": 1, "1080p": 2, "4K": 3}

func minQualityRank(minq string) int {
	switch minq {
	case "720":
		return 1
	case "1080":
		return 2
	case "2160":
		return 3
	}
	return 0
}

// poolPage — страница из пула с учётом exclude юзера и его фильтров раздач
// (minq/озвучки): пул собирается без озвучечных фильтров юзера, поэтому
// они применяются при нарезке — иначе SD и без-озвучковые карточки
// протекают в отфильтрованный топ
func poolPage(variant string, page, feedSize int, exclude map[int]bool, minq, voices string) ([]map[string]any, int, error) {
	all, err := getPool(variant)
	if err != nil {
		return nil, 0, err
	}

	voiceSet := map[string]bool{}
	for _, v := range strings.Split(voices, ",") {
		if v = strings.TrimSpace(v); v != "" {
			voiceSet[v] = true
		}
	}
	needRank := minQualityRank(minq)

	filtered := make([]map[string]any, 0, len(all))
	for _, el := range all {
		if idf, ok := el["id"].(float64); ok && exclude != nil && exclude[int(idf)] {
			continue
		}
		q, _ := el["quality"].(string)
		if cardQualityRank[q] < needRank {
			continue
		}
		if len(voiceSet) > 0 {
			vv, _ := el["voice"].(string)
			if !voiceSet[vv] {
				continue
			}
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
