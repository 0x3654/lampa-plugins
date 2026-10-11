package main

import (
	"strings"
	"testing"
	"time"
)

// ---------- блоки главной (ajax.php?mode=pop|new|newest)

func TestParseSvSidebar(t *testing.T) {
	items := parseSvSidebar(fixture(t, "sv-sidebar.html"))

	check(t, "items >= 20", len(items) >= 20, len(items))

	first := items[0]
	check(t, "path", strings.HasPrefix(first.Path, "/serial-"), first.Path)
	check(t, "id", first.ID > 0, first.ID)
	check(t, "ru", first.Ru != "", first.Ru)
	check(t, "poster", strings.HasPrefix(first.Poster, "https://cdn.svstat.net/oblojka/"), first.Poster)
	check(t, "info", first.Info != "", first.Info)
	// первый айтем снапшота — «Невский», «Сезон 8»: номер сезона из блока
	check(t, "season", first.Season == 8, first.Season)

	// у части айтемов есть оригинальное название (rside-t_en)
	origs := 0
	for _, it := range items {
		if it.Orig != "" {
			origs++
		}
	}
	check(t, "origs > 0", origs > 0, origs)

	// мемо id→path заполнился
	check(t, "path memo", svPathByID(first.ID) == first.Path, svPathByID(first.ID))
}

// ---------- «На чем остановились» (/resume)

func TestParseSvResume(t *testing.T) {
	auth, uid, tabs := parseSvResume(fixture(t, "sv-resume.html"))

	check(t, "auth", auth)
	check(t, "uid", uid > 0, uid)

	items := tabs["new"]
	check(t, "marks-new", len(items) == 1, len(items))

	it := items[0]
	check(t, "id", it.ID == 49803, it.ID)
	check(t, "ru", it.Ru == "Кэрри", it.Ru)
	check(t, "watched", it.Watched == 1, it.Watched)
	check(t, "total", it.Total == 8, it.Total)
	check(t, "last", strings.Contains(it.Last, "LostFilm"), it.Last)
	check(t, "path", strings.HasPrefix(it.Path, "/serial-49803-"), it.Path)
	check(t, "poster", strings.Contains(it.Poster, "oblojka/49803.jpg"), it.Poster)
	check(t, "updated", it.Updated != "", it.Updated)

	for _, tab := range []string{"nonew", "want", "watched"} {
		check(t, "tab "+tab+" пуст", len(tabs[tab]) == 0, len(tabs[tab]))
	}
}

// гость видит те же вкладки без закладок — auth обязан быть false
func TestParseSvResumeGuest(t *testing.T) {
	auth, uid, tabs := parseSvResume(strings.ReplaceAll(fixture(t, "sv-resume.html"),
		`href="/profile/2365606"`, `href="/profile/"`))

	check(t, "guest не auth", !auth)
	check(t, "guest uid 0", uid == 0, uid)
	check(t, "guest пусто", len(tabs["new"]) == 1) // айтемы парсятся и без авторизации
}

// ---------- страница сезона

func TestParseSvSerialSingle(t *testing.T) {
	s, err := parseSvSerial(fixture(t, "sv-serial.html"))
	if err != nil {
		t.Fatal(err)
	}

	check(t, "id", s.ID == 49803, s.ID)
	check(t, "serial_id", s.SerialID == 34015, s.SerialID)
	check(t, "ru", s.Ru == "Кэрри", s.Ru)
	check(t, "orig", s.Orig == "Carrie", s.Orig)
	check(t, "year", s.Year == 2026, s.Year)
	check(t, "seasons", len(s.Seasons) == 1, len(s.Seasons))
	check(t, "current", s.Seasons[0].Current)
	check(t, "path", s.Path == s.Seasons[0].Path, s.Path)
	check(t, "voices >= 2", len(s.Voices) >= 2, len(s.Voices))
	check(t, "episodes", s.Episodes == 8, s.Episodes) // снапшот снят с отметкой на 1-й серии
	check(t, "mark_seria", s.MarkSeria == 1, s.MarkSeria)
	check(t, "poster", strings.Contains(s.Poster, "oblojka/49803.jpg"), s.Poster)
}

func TestParseSvSerialMulti(t *testing.T) {
	s, err := parseSvSerial(fixture(t, "sv-got.html"))
	if err != nil {
		t.Fatal(err)
	}

	check(t, "id", s.ID == 2219, s.ID)
	check(t, "seasons 8", len(s.Seasons) == 8, len(s.Seasons))
	check(t, "ru", s.Ru == "Игра престолов", s.Ru)
	check(t, "orig", s.Orig == "Game of Thrones", s.Orig)
	check(t, "year", s.Year == 2011, s.Year)

	current := 0
	for _, ref := range s.Seasons {
		if ref.Current {
			current = ref.ID
		}
	}
	check(t, "current id", current == 2219, current)

	// номера сезонов 1..8 из подписей ссылок
	nums := map[int]bool{}
	for _, ref := range s.Seasons {
		nums[ref.Season] = true
	}
	for n := 1; n <= 8; n++ {
		check(t, "сезон в списке", nums[n], n)
	}

	check(t, "mark пуст без сессии", s.MarkSeria == 0, s.MarkSeria)
}

// ---------- кэш года сезона (матчинг TMDB «имя + год»)

func TestSvYearCache(t *testing.T) {
	svYearC[49803] = svYearEntry{ts: time.Now(), year: 2026}
	svYearC[1] = svYearEntry{ts: time.Now().Add(-31 * 24 * time.Hour), year: 2020}

	check(t, "год из кэша", svYearCached(49803) == 2026)
	check(t, "неизвестный → 0", svYearCached(2) == 0)
	check(t, "просроченный → 0", svYearCached(1) == 0, svYearCached(1))
}

// ---------- подсказки поиска (autocomplete.php)

func TestParseSvAutocomplete(t *testing.T) {
	items := parseSvAutocomplete(fixture(t, "sv-ac.json"))

	check(t, "items >= 5", len(items) >= 5, len(items))

	first := items[0]
	check(t, "id", first.ID == 2219, first.ID)
	check(t, "ru", first.Ru == "Игра престолов", first.Ru)
	check(t, "orig", first.Orig == "Game of Thrones", first.Orig)
	check(t, "season", first.Season == 1, first.Season)
	check(t, "path", strings.HasPrefix(first.Path, "/serial-2219-"), first.Path)
}
