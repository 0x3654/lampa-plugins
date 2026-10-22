package main

import "testing"

// TestJacredItemFullPipeline — item из реального title JacRed проходит
// все условия findReleaseList2: год, имена, сидеры, русский, качество
func TestJacredItemFullPipeline(t *testing.T) {
	title := "Глубина / Below / Сезон: 1 / Серии: 1-6 из 6 (Джейми Чайлдс) [2026, Канада, ужасы, триллер, драма, детектив, WEB-DL 1080p] Dub (Movie Dubbing) + Dub (Ukr) + Original + Sub (Rus, Eng, Chi, Heb, Ron, Ukr) | [Дубляж Movie Dubbing].rus"

	attrs := []struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	}{
		{"magneturl", "magnet:?xt=urn:btih:35026FD970F3A42CED7AD39355188B9C6D3232BE"},
		{"seeders", "2083"},
		{"leechers", "1006"},
		{"size", "12219181957"},
		{"site", "rutracker"},
		{"year", "2026"},
	}

	it := jacredItemFromTitle(title, attrs)

	if it.Ru != "Глубина" || it.Year != 2026 {
		t.Fatalf("ru=%q year=%d", it.Ru, it.Year)
	}
	if it.Seeders != 2083 || it.Magnet == "" {
		t.Fatalf("атрибуты: seeders=%d magnet=%q", it.Seeders, it.Magnet)
	}
	if it.Source != "jacred:rutracker" {
		t.Fatalf("source=%q", it.Source)
	}
	if it.Quality != "1080" {
		t.Fatalf("quality=%q", it.Quality)
	}
	if !it.Dub {
		t.Fatal("дубляж не распознан из «Dub (Movie Dubbing)»")
	}
	if it.SubOnly {
		t.Fatal("ложно SubOnly")
	}

	// условия findReleaseList2 (query=Глубина, orig=Below, year=2026, tv):
	nq, no := normName("Глубина"), normName("Below")
	ruMatch := normName(it.Ru) == nq && (no == "" || it.Orig == "" || normName(it.Orig) == no)
	origMatch := no != "" && normName(it.Orig) == no
	if !ruMatch && !origMatch {
		t.Fatalf("имена не матчились: ru=%q(orig=%q) vs q=%q no=%q", it.Ru, it.Orig, nq, no)
	}
	if it.Year == 0 || abs(2026-it.Year) > 6 {
		t.Fatalf("год: item=%d want=2026", it.Year)
	}
	if it.Seeders <= 0 {
		t.Fatal("сидеры")
	}

	kept := filterItems([]Item{it}, "1080", "all")
	kept = filterRussian(kept, "1")
	if len(kept) == 0 {
		t.Fatal("вырезан фильтрами качества/русского")
	}
}
