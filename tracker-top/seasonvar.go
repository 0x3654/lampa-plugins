// seasonvar.go — прокси seasonvar.ru для топа «Seasonvar» в Лампе.
//
// У сайта нет CORS, поэтому всё ходит через нас. Сессия — кука svid1
// (30 дней) — живёт на устройстве: /seasonvar/login единожды логинится
// и отдаёт её клиенту, дальше она приходит параметром sk. Креды на
// сервере не хранятся. Личные эндпоинты отличают «пусто» от «сессия
// умерла» по ссылке /profile/<uid> в ответе.
//
// GET /seasonvar/login?login=&passw=      → {ok, sk} — svid1 для устройства
// GET /seasonvar/home?mode=pop|new|newest → {items} — блоки главной (общий кэш)
// GET /seasonvar/tracking?sk=             → {ok, uid, tabs:{new,nonew,want,watched}}
// GET /seasonvar/serial?path=[&id=][&sk=] → сезон: сезоны/озвучки/отметка
// GET /seasonvar/search?q=                → {items} — autocomplete, нормализованный
// GET /seasonvar/mark?sk=&mode=&id=…      → транзит jsonMark.php (без кэша)
//
// URL сезона строгий: /serial-<id>-<slug>.html, слаг менять нельзя (404) —
// айтемы носят точный path, сервер ведёт мемо id→path.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var seasonvarBase = env("SEASONVAR_BASE", "https://seasonvar.ru")

// сайт не жаловался на частоту, но ломиться пачками незачем
var svSem = make(chan struct{}, 2)

// SvItem — сезон сериала в терминах seasonvar (ID — именно season id,
// не сериала: отметки и страницы живут на нём)
type SvItem struct {
	ID     int    `json:"id"`
	Path   string `json:"path"` // /serial-<id>-<slug>.html — слаг строгий
	Ru     string `json:"ru"`
	Orig   string `json:"orig,omitempty"`
	Year   int    `json:"year,omitempty"`
	Season int    `json:"season,omitempty"` // номер сезона, 0 — неизвестен
	Info   string `json:"info,omitempty"`   // «19-20 серия» / «Сезон 8» с озвучкой
	Poster string `json:"poster,omitempty"`
	// поля «Отслеживаемых» (/resume)
	Watched int    `json:"watched,omitempty"` // «1 из 8» → 1
	Total   int    `json:"total,omitempty"`   // …из 8 → 8
	Last    string `json:"last,omitempty"`    // «1 серия от LostFilm»
	Updated string `json:"updated,omitempty"` // дата в списке обновлений
}

type SvSeasonRef struct {
	ID      int    `json:"id"`
	Path    string `json:"path"`
	Season  int    `json:"season"` // номер сезона из подписи ссылки
	Current bool   `json:"current,omitempty"`
}

type SvVoice struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SvSerial — страница сезона: сам сезон + список всех сезонов сериала
type SvSerial struct {
	SvItem
	SerialID  int           `json:"serial_id"`
	Seasons   []SvSeasonRef `json:"seasons"`
	Voices    []SvVoice     `json:"voices,omitempty"`
	Episodes  int           `json:"episodes,omitempty"`   // серий текущего сезона («Стандартный»)
	MarkSeria int           `json:"mark_seria,omitempty"` // просмотрено до серии (var mark)
	MarkTran  int           `json:"mark_tran,omitempty"`
}

// ---------- HTTP

// svFetch — GET/POST к seasonvar с кукой сессии и заголовками «сайтового»
// AJAX (X-Requested-With + Referer ждут ajax.php/jsonMark.php)
func svFetch(method, path, form, sk string) (string, error) {
	svSem <- struct{}{}
	defer func() { <-svSem }()

	var body io.Reader
	if form != "" {
		body = strings.NewReader(form)
	}
	req, err := http.NewRequest(method, seasonvarBase+path, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	if form != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Referer", seasonvarBase+"/")
	if strings.Contains(path, "ajax.php") || strings.Contains(path, "jsonMark.php") {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	if sk != "" {
		req.Header.Set("Cookie", "svid1="+sk)
	}

	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("seasonvar %s: HTTP %d", path, resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(raw), nil // сайт в utf-8
}

// svLogin — POST /?mod=login: редирект не преследуем, svid1 берём из
// ответа (форма без CSRF; неверный пароль — просто без куки)
func svLogin(login, passw string) (string, error) {
	form := url.Values{"login": {login}, "password": {passw}}.Encode()

	svSem <- struct{}{}
	defer func() { <-svSem }()

	req, err := http.NewRequest("POST", seasonvarBase+"/?mod=login", strings.NewReader(form))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", seasonvarBase+"/")

	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	for _, c := range resp.Cookies() {
		if c.Name == "svid1" && c.Value != "" {
			return c.Value, nil
		}
	}
	return "", errors.New("seasonvar: не отдал svid1 — неверный логин/пароль?")
}

// ---------- мемо id→path: слаг строгий, по одному id страницу не собрать

var (
	svPathMu   sync.Mutex
	svPathMemo = map[int]string{}
)

func svRememberPath(id int, path string) {
	if id == 0 || path == "" {
		return
	}
	svPathMu.Lock()
	svPathMemo[id] = path
	svPathMu.Unlock()
}

func svPathByID(id int) string {
	svPathMu.Lock()
	defer svPathMu.Unlock()
	return svPathMemo[id]
}

// ---------- регулярки

var (
	svTagRe   = regexp.MustCompile(`<[^>]+>`)
	svSpaceRe = regexp.MustCompile(`\s+`)

	svSideARe     = regexp.MustCompile(`(?s)<a href="(/serial-(\d+)-[^"]+)" class="pst rside-p[^"]*">(.*?)</a>`)
	svPosterRe    = regexp.MustCompile(`src="//(cdn\.svstat\.net/oblojka/[^"]+)"`)
	svSideTRe     = regexp.MustCompile(`(?s)<div class="rside-t">([^<]*)</div>`)
	svSideTEnRe   = regexp.MustCompile(`(?s)<div class="rside-t_en">([^<]*)</div>`)
	svSideSSRe    = regexp.MustCompile(`(?s)<div class="rside-ss">(.*?)</div>`)
	svSeasonNumRe = regexp.MustCompile(`[Сс]езон\s*(\d+)`)

	svProfileRe   = regexp.MustCompile(`href="/profile/(\d+)"`)
	svMarksSelRe  = regexp.MustCompile(`data-marksel="(\d+)"`)
	svMarksHrefRe = regexp.MustCompile(`href="(/serial-(\d+)-[^"]+)"`)
	svMarksImgRe  = regexp.MustCompile(`oblojka/small/(\d+)\.jpg`)
	svMarksNameRe = regexp.MustCompile(`(?s)<div class="pgs-marks-name">([^<]*)</div>`)
	svMarksCurRe  = regexp.MustCompile(`(?s)<div class="pgs-marks-current">(.*?)</div>`)
	svMarksLastRe = regexp.MustCompile(`(?s)<div class="pgs-marks-last">(.*?)</div>`)
	svMarksMessRe = regexp.MustCompile(`(?s)<div class="pgs-marks-mess">(.*?)</div>`)
	svOfTotalRe   = regexp.MustCompile(`(\d+)\s*из\s*(\d+)`)

	svSerialIDRe = regexp.MustCompile(`data-id-season="(\d+)"\s+data-id-serial="(\d+)"`)
	svOrigRe     = regexp.MustCompile(`Оригинал:\s*(?:<span>)?([^<]*)`)
	svYearRe     = regexp.MustCompile(`Вышел:\s*(?:<span>)?(\d{4})`)
	svOgImageRe  = regexp.MustCompile(`og:image" content="([^"]+)"`)
	svSeasonARe  = regexp.MustCompile(`(?s)<a href="(/serial-(\d+)-[^"]+)">\s*(.*?)</a>`)
	svVoiceRe    = regexp.MustCompile(`data-translate="(\d+)"[^>]*>([^<]+)</li>`)
	svEpisodesRe = regexp.MustCompile(`(?s)arEpisodes\s*=\s*(\{.*?\});`)
	svMarkRe     = regexp.MustCompile(`(?s)var mark = \{.*?'href':\s*'([^']*)'.*?'trans':\s*'([^']*)'`)
	svRewindRe   = regexp.MustCompile(`#rewind=(\d+)_`)
	// хвост «N сезон»/«(N сезон)» в подписях: «Game of Thrones 1 сезон»,
	// «MobLand (2 сезон)» — номер бывает и до, и после слова
	svSeasonNum2Re = regexp.MustCompile(`(\d+)\s+[Сс]езон`)
	svTrailSeaRe   = regexp.MustCompile(`\s*\(\s*\d+\s+[Сс]езон\s*\)\s*$|\s*\d+\s+[Сс]езон\s*$`)

	svAutocompleteSeaRe = regexp.MustCompile(`\((\d+)\s+[Сс]езон\)`)
)

// svSeasonNumberOf — номер сезона из подписи: «Сезон 8» или «1 сезон»
func svSeasonNumberOf(s string) int {
	if n := svSeasonNumRe.FindStringSubmatch(s); n != nil {
		return atoiDefault(n[1])
	}
	if n := svSeasonNum2Re.FindStringSubmatch(s); n != nil {
		return atoiDefault(n[1])
	}
	return 0
}

func svStrip(s string) string {
	return strings.TrimSpace(svSpaceRe.ReplaceAllString(svTagRe.ReplaceAllString(s, " "), " "))
}

// ---------- парсинг

// parseSvSidebar — фрагмент ajax.php?mode=…: карточки блока главной
func parseSvSidebar(body string) []SvItem {
	var items []SvItem
	for _, m := range svSideARe.FindAllStringSubmatch(body, -1) {
		it := SvItem{Path: m[1], ID: atoiDefault(m[2])}
		inner := m[3]

		if p := svPosterRe.FindStringSubmatch(inner); p != nil {
			it.Poster = "https://" + p[1]
		}
		if t := svSideTRe.FindStringSubmatch(inner); t != nil {
			it.Ru = strings.TrimSpace(t[1])
		}
		if t := svSideTEnRe.FindStringSubmatch(inner); t != nil {
			it.Orig = strings.TrimSpace(t[1])
		}
		if ss := svSideSSRe.FindStringSubmatch(inner); ss != nil {
			it.Info = svStrip(ss[1])
			it.Season = svSeasonNumberOf(it.Info)
		}

		svRememberPath(it.ID, it.Path)
		items = append(items, it)
	}
	return items
}

// parseSvResume — «На чем остановились»: все четыре вкладки одной страницей.
// Авторизацию определяем по /profile/<uid> в меню (гостю отдаются те же
// пустые вкладки — иначе «сессия умерла» выглядит как «закладок нет»)
func parseSvResume(body string) (bool, int, map[string][]SvItem) {
	uid := 0
	if m := svProfileRe.FindStringSubmatch(body); m != nil {
		uid = atoiDefault(m[1])
	}
	auth := uid > 0

	tabs := map[string][]SvItem{}
	sections := strings.Split(body, `<li data-tabr="marks-`)
	for _, sec := range sections[1:] {
		tab := strings.SplitN(sec, `"`, 2)[0] // new | nonew | want | watched | filter
		if tab == "filter" {
			continue
		}
		// вкладка заканчивается своим </li>: без обрезки последняя секция
		// доезжает до конца страницы и захватывает чужие виджеты с data-marksel
		if j := strings.Index(sec, "</li>"); j >= 0 {
			sec = sec[:j]
		}
		for _, chunk := range strings.Split(sec, `<div class="pgs-marks-el"`) {
			if chunk == "" {
				continue
			}
			it := SvItem{}
			if m := svMarksSelRe.FindStringSubmatch(chunk); m != nil {
				it.ID = atoiDefault(m[1])
			}
			if m := svMarksHrefRe.FindStringSubmatch(chunk); m != nil {
				it.Path = m[1]
				if it.ID == 0 {
					it.ID = atoiDefault(m[2])
				}
			}
			if it.ID == 0 {
				continue
			}
			if p := svMarksImgRe.FindStringSubmatch(chunk); p != nil {
				it.Poster = "https://cdn.svstat.net/oblojka/" + p[1] + ".jpg"
			}
			if m := svMarksNameRe.FindStringSubmatch(chunk); m != nil {
				it.Ru = strings.TrimSpace(m[1])
			}
			if m := svMarksCurRe.FindStringSubmatch(chunk); m != nil {
				if o := svOfTotalRe.FindStringSubmatch(svStrip(m[1])); o != nil {
					it.Watched = atoiDefault(o[1])
					it.Total = atoiDefault(o[2])
				}
			}
			if m := svMarksLastRe.FindStringSubmatch(chunk); m != nil {
				it.Last = svStrip(m[1])
			}
			if m := svMarksMessRe.FindStringSubmatch(chunk); m != nil {
				it.Updated = svStrip(m[1])
			}

			svRememberPath(it.ID, it.Path)
			tabs[tab] = append(tabs[tab], it)
		}
	}
	return auth, uid, tabs
}

// parseSvSerial — страница сезона: сам сезон, все сезоны сериала, озвучки,
// число серий и текущая отметка пользователя
func parseSvSerial(body string) (SvSerial, error) {
	var s SvSerial

	if m := svSerialIDRe.FindStringSubmatch(body); m != nil {
		s.ID = atoiDefault(m[1])
		s.SerialID = atoiDefault(m[2])
	} else {
		return s, errors.New("seasonvar: страница не похожа на сезон (нет data-id-season)")
	}

	// список сезонов: от pgs-seaslist до content-wrap (внутри только ul/li)
	seas := body
	if i := strings.Index(seas, `pgs-seaslist`); i >= 0 {
		seas = seas[i:]
		if j := strings.Index(seas, `content-wrap`); j >= 0 {
			seas = seas[:j]
		}
	}
	var currentTitle string
	for _, m := range svSeasonARe.FindAllStringSubmatch(seas, -1) {
		title := svStrip(m[3])
		ref := SvSeasonRef{Path: m[1], ID: atoiDefault(m[2])}
		if strings.Contains(title, ">>>") {
			ref.Current = true
			currentTitle = strings.TrimSpace(strings.ReplaceAll(title, ">>>", ""))
		}
		ref.Season = svSeasonNumberOf(title)
		svRememberPath(ref.ID, ref.Path)
		s.Seasons = append(s.Seasons, ref)
	}

	// ru/orig — из подписи текущего сезона «Сериал Ру / Orig N сезон»,
	// оригинал уточняет блок «Оригинал:» на странице
	if currentTitle != "" {
		base := strings.TrimSpace(strings.TrimPrefix(currentTitle, "Сериал"))
		if i := strings.Index(base, "/"); i >= 0 {
			s.Ru = strings.TrimSpace(base[:i])
			s.Orig = svTrailSeaRe.ReplaceAllString(strings.TrimSpace(base[i+1:]), "")
		} else {
			s.Ru = base
		}
	}
	if m := svOrigRe.FindStringSubmatch(body); m != nil {
		if orig := strings.TrimSpace(m[1]); orig != "" {
			s.Orig = orig
		}
	}
	if m := svYearRe.FindStringSubmatch(body); m != nil {
		s.Year = atoiDefault(m[1])
	}
	if m := svOgImageRe.FindStringSubmatch(body); m != nil {
		s.Poster = m[1]
		if strings.HasPrefix(s.Poster, "//") {
			s.Poster = "https:" + s.Poster
		}
	}
	for _, ref := range s.Seasons {
		if ref.Current {
			s.Path = ref.Path
			s.Season = ref.Season
		}
	}
	if s.Path == "" {
		s.Path = svPathByID(s.ID)
	}

	for _, m := range svVoiceRe.FindAllStringSubmatch(body, -1) {
		s.Voices = append(s.Voices, SvVoice{ID: atoiDefault(m[1]), Name: strings.TrimSpace(m[2])})
	}

	// arEpisodes: счёт серий текущего сезона по переводу «Стандартный» (0);
	// у промо/трейлеров n нечисловой — не считаем
	if m := svEpisodesRe.FindStringSubmatch(body); m != nil {
		var eps map[string]map[string]struct {
			N    string `json:"n"`
			Next string `json:"next"`
		}
		if err := json.Unmarshal([]byte(m[1]), &eps); err == nil {
			for _, ep := range eps["0"] {
				if _, err := strconv.Atoi(ep.N); err == nil {
					s.Episodes++
				}
			}
		}
	}

	if m := svMarkRe.FindStringSubmatch(body); m != nil {
		s.MarkTran = atoiDefault(m[2])
		if r := svRewindRe.FindStringSubmatch(m[1]); r != nil {
			s.MarkSeria = atoiDefault(r[1])
		}
	}

	svRememberPath(s.ID, s.Path)
	return s, nil
}

// parseSvAutocomplete — подсказки поиска: параллельные массивы → айтемы
func parseSvAutocomplete(body string) []SvItem {
	var data struct {
		Suggestions struct {
			Valu []string `json:"valu"`
		} `json:"suggestions"`
		Data []string `json:"data"`
		ID   []string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil
	}
	var items []SvItem
	for i := range data.Data {
		if i >= len(data.ID) || i >= len(data.Suggestions.Valu) {
			break
		}
		it := SvItem{ID: atoiDefault(data.ID[i]), Path: "/" + data.Data[i]}
		title := data.Suggestions.Valu[i]
		if j := strings.Index(title, "/"); j >= 0 {
			it.Ru = strings.TrimSpace(title[:j])
			it.Orig = svTrailSeaRe.ReplaceAllString(strings.TrimSpace(title[j+1:]), "")
		} else {
			it.Ru = title
		}
		if n := svAutocompleteSeaRe.FindStringSubmatch(title); n != nil {
			it.Season = atoiDefault(n[1])
		}
		svRememberPath(it.ID, it.Path)
		items = append(items, it)
	}
	return items
}

// ---------- год сезона: в списках seasonvar года нет совсем, он только
// на странице сезона («Вышел: …»). Год не меняется — добираем страницей
// и держим месяц; без года матчинг TMDB по одному имени ненадёжен
// (кейс «Ронин»: русский сериал ловился фильмом 1998)

type svYearEntry struct {
	ts   time.Time
	year int
}

var (
	svYearMu sync.Mutex
	svYearC  = map[int]svYearEntry{}
)

// svYearCached — год из кэша без похода в сеть (для «Отслеживаемых»)
func svYearCached(id int) int {
	svYearMu.Lock()
	defer svYearMu.Unlock()

	if e, ok := svYearC[id]; ok && time.Since(e.ts) < 30*24*time.Hour {
		return e.year
	}
	return 0
}

func svYearOf(id int, path string) int {
	if y := svYearCached(id); y != 0 {
		return y
	}
	if path == "" {
		return 0
	}

	s, err := svSerialPage(path, "")
	if err != nil || s.Year == 0 {
		return 0
	}

	svYearMu.Lock()
	svYearC[id] = svYearEntry{ts: time.Now(), year: s.Year}
	svYearMu.Unlock()

	return s.Year
}

// svFillYears — параллельно добираем год блоку главной: холодный проход
// ~49 страниц под семафором 4 — единожды несколько секунд, дальше год
// из месячного кэша, страницу дёргаем только у новых сериалов
func svFillYears(items []SvItem) {
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for i := range items {
		if items[i].Year != 0 || items[i].Path == "" {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			items[i].Year = svYearOf(items[i].ID, items[i].Path)
		}(i)
	}

	wg.Wait()
}

// ---------- кэш (in-memory, ключи с cacheVer — инвариант репо)

type svCacheEntry[T any] struct {
	ts  time.Time
	val T
}

type svTracking struct {
	OK   bool                `json:"ok"`
	UID  int                 `json:"uid,omitempty"`
	Tabs map[string][]SvItem `json:"tabs"`
}

var (
	svHomeMu    sync.Mutex
	svHomeCache = map[string]svCacheEntry[[]SvItem]{}
	svSerialMu  sync.Mutex
	svSerialC   = map[string]svCacheEntry[SvSerial]{}
	svTrackMu   sync.Mutex
	svTrackC    = map[string]svCacheEntry[svTracking]{}
)

func svHomeBlock(mode string) ([]SvItem, error) {
	// y2 — список обогащён годами (матчинг TMDB «имя + год»)
	key := cacheVer + "|svhome2|" + mode
	svHomeMu.Lock()
	if e, ok := svHomeCache[key]; ok && time.Since(e.ts) < 30*time.Minute {
		svHomeMu.Unlock()
		return e.val, nil
	}
	svHomeMu.Unlock()

	body, err := svFetch("POST", "/ajax.php?mode="+mode, "ganre=&country=&block=0&main=1", "")
	if err != nil {
		return nil, err
	}
	items := parseSvSidebar(body)
	if len(items) == 0 {
		return nil, errors.New("seasonvar: пустой блок " + mode)
	}

	// год обязателен для матчинга; кэшируем уже обогащённый список
	svFillYears(items)

	svHomeMu.Lock()
	svHomeCache[key] = svCacheEntry[[]SvItem]{ts: time.Now(), val: items}
	svHomeMu.Unlock()
	return items, nil
}

func svSerialPage(path, sk string) (SvSerial, error) {
	// с сессией — всегда свежая страница (отметка пользователя);
	// без сессии — 10 минут кэша
	key := cacheVer + "|svserial|" + path
	if sk == "" {
		svSerialMu.Lock()
		if e, ok := svSerialC[key]; ok && time.Since(e.ts) < 10*time.Minute {
			svSerialMu.Unlock()
			return e.val, nil
		}
		svSerialMu.Unlock()
	}

	body, err := svFetch("GET", path, "", sk)
	if err != nil {
		return SvSerial{}, err
	}
	s, err := parseSvSerial(body)
	if err != nil {
		return SvSerial{}, err
	}

	if sk == "" {
		svSerialMu.Lock()
		svSerialC[key] = svCacheEntry[SvSerial]{ts: time.Now(), val: s}
		svSerialMu.Unlock()
	}
	return s, nil
}

func svTrackingList(sk string) (svTracking, error) {
	key := cacheVer + "|svtrack|" + sk
	svTrackMu.Lock()
	if e, ok := svTrackC[key]; ok && time.Since(e.ts) < time.Minute {
		svTrackMu.Unlock()
		return e.val, nil
	}
	svTrackMu.Unlock()

	body, err := svFetch("GET", "/resume", "", sk)
	if err != nil {
		return svTracking{}, err
	}
	auth, uid, tabs := parseSvResume(body)

	// год из кэша без похода в сеть — страницей сезона блок не ждём
	for _, list := range tabs {
		for i := range list {
			list[i].Year = svYearCached(list[i].ID)
		}
	}

	res := svTracking{OK: auth, UID: uid, Tabs: tabs}

	svTrackMu.Lock()
	svTrackC[key] = svCacheEntry[svTracking]{ts: time.Now(), val: res}
	svTrackMu.Unlock()
	return res, nil
}

// ---------- обработчики

var svPathOKRe = regexp.MustCompile(`^/serial-\d+-[^"\s]+\.html$`)

func seasonvarHandlers(mux *http.ServeMux) {
	// логин: креды приходят GET-параметрами (сетевой стек Лампы не умеет
	// надёжно POST), уходят в форму seasonvar и назад не сохраняются
	mux.HandleFunc("GET /seasonvar/login", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		login := param(q, "login", "")
		passw := param(q, "passw", "")
		if login == "" || passw == "" {
			writeJSON(w, 400, map[string]any{"ok": false, "error": "login и passw обязательны"})
			return
		}
		sk, err := svLogin(login, passw)
		if err != nil {
			writeJSON(w, 401, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "sk": sk})
	})

	// блоки главной: pop — «Топ», newest — «Новинки», new — «Обновления»
	mux.HandleFunc("GET /seasonvar/home", func(w http.ResponseWriter, r *http.Request) {
		mode := param(r.URL.Query(), "mode", "pop")
		if mode != "pop" && mode != "new" && mode != "newest" {
			writeJSON(w, 400, map[string]string{"error": "mode must be one of pop|new|newest"})
			return
		}
		items, err := svHomeBlock(mode)
		if err != nil {
			writeJSON(w, 502, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})

	// отслеживаемые: ok=false — сессия умерла, клиент перелогинивается
	mux.HandleFunc("GET /seasonvar/tracking", func(w http.ResponseWriter, r *http.Request) {
		sk := param(r.URL.Query(), "sk", "")
		if sk == "" {
			writeJSON(w, 400, map[string]string{"error": "sk required"})
			return
		}
		res, err := svTrackingList(sk)
		if err != nil {
			writeJSON(w, 502, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, res)
	})

	mux.HandleFunc("GET /seasonvar/serial", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sk := param(q, "sk", "")
		path := param(q, "path", "")
		if path == "" {
			// слаг строгий: по одному id берём из мемо (появляется после
			// первого же блока/поиска/отслеживаемых)
			if id, err := strconv.Atoi(param(q, "id", "")); err == nil {
				path = svPathByID(id)
			}
		}
		if path == "" || !svPathOKRe.MatchString(path) {
			writeJSON(w, 400, map[string]string{"error": "нужен path=/serial-<id>-<slug>.html"})
			return
		}
		s, err := svSerialPage(path, sk)
		if err != nil {
			writeJSON(w, 502, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, s)
	})

	mux.HandleFunc("GET /seasonvar/search", func(w http.ResponseWriter, r *http.Request) {
		q := param(r.URL.Query(), "q", "")
		if q == "" {
			writeJSON(w, 400, map[string]string{"error": "q required"})
			return
		}
		body, err := svFetch("GET", "/autocomplete.php?query="+url.QueryEscape(q), "", "")
		if err != nil {
			writeJSON(w, 502, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"items": parseSvAutocomplete(body)})
	})

	// запись отметки — транзит jsonMark.php; формы подсмотрены в pg.marks.min.js
	mux.HandleFunc("GET /seasonvar/mark", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sk := param(q, "sk", "")
		id := param(q, "id", "")
		if sk == "" || id == "" {
			writeJSON(w, 400, map[string]string{"error": "sk и id обязательны"})
			return
		}

		var form string
		switch param(q, "mode", "pause") {
		case "pause": // отметка на серии
			form = "pauseadd=1&id=" + url.QueryEscape(id) + "&seria=" + url.QueryEscape(param(q, "seria", "1"))
			if v := param(q, "tran", ""); v != "" {
				form += "&tran=" + url.QueryEscape(v)
			}
			if m := param(q, "minute", ""); m != "" {
				form += "&minute=" + url.QueryEscape(m) + "&second=" + url.QueryEscape(param(q, "second", "0"))
			}
		case "season": // сезон просмотрен целиком
			form = "id=" + url.QueryEscape(id) + "&seria=-2&minute=0&second=0&watched=1"
		case "want": // хочу посмотреть
			form = "id=" + url.QueryEscape(id) + "&seria=-1&minute=0&second=0&wanttosee=1"
		case "del": // снять отметку
			form = "delId=" + url.QueryEscape(id)
		default:
			writeJSON(w, 400, map[string]string{"error": "mode must be one of pause|season|want|del"})
			return
		}

		body, err := svFetch("POST", "/jsonMark.php", form, sk)
		if err != nil {
			writeJSON(w, 502, map[string]string{"error": err.Error()})
			return
		}
		var res map[string]any
		if err := json.Unmarshal([]byte(body), &res); err != nil {
			writeJSON(w, 502, map[string]string{"error": "seasonvar ответил не JSON"})
			return
		}
		// после записи личные кэши протухли
		svTrackMu.Lock()
		delete(svTrackC, cacheVer+"|svtrack|"+sk)
		svTrackMu.Unlock()
		writeJSON(w, 200, res)
	})
}
