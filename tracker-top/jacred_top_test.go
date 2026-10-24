package main

import (
	"encoding/json"
	"os"
	"testing"
)

// смоук маппинга нативного топа агрегатора /api/v1.0/top → Item:
// качество из пикселей, «Дубляж» из канонических voices, сериальность
// из type/seasons, битые строки (best=null) выпадают
func TestTopRowsToItems(t *testing.T) {
	raw, err := os.ReadFile("tests/jr-top-sample.json")
	if err != nil {
		t.Skip("нет фикстуры tests/jr-top-sample.json")
	}

	var top topResp
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("разбор фикстуры: %v", err)
	}

	items := topRowsToItems(top.Rows)

	if len(items) != 2 {
		t.Fatalf("ожидали 2 item (битая строка выпадает), получили %d", len(items))
	}

	movie := items[0]
	if movie.Ru != "Глубина" || movie.Orig != "Below" || movie.Year != 2026 {
		t.Fatalf("фильм: имя/год: %+v", movie)
	}
	if movie.Quality != "1080" {
		t.Fatalf("фильм: качество 1080 из quality=1080, получили %q", movie.Quality)
	}
	if !movie.Dub {
		t.Fatal("фильм: Дубляж должен подняться из voices [\"Дубляж\"]")
	}
	if movie.Season {
		t.Fatal("фильм: Season=false (type=movie, сезонов нет)")
	}
	if movie.Seeders != 142 || movie.Size != 8450000000 || movie.SizeText != "7.9 GB" {
		t.Fatalf("фильм: сиды/размер: %d %d %q", movie.Seeders, movie.Size, movie.SizeText)
	}
	if movie.Source != "jacred:rutor" || movie.Magnet != "magnet:?xt=urn:btih:TEST1" {
		t.Fatalf("фильм: источник/магнит: %q %q", movie.Source, movie.Magnet)
	}

	tv := items[1]
	if !tv.Season {
		t.Fatal("сериал: Season=true (type=tv и seasons=[1])")
	}
	if tv.Quality != "720" {
		t.Fatalf("сериал: качество 720, получили %q", tv.Quality)
	}
	if tv.Voice != "Гоблин" {
		t.Fatalf("сериал: озвучка студией из voices, получили %q", tv.Voice)
	}
}

func TestQualityFromInt(t *testing.T) {
	cases := map[int]string{
		2160: "2160", 2180: "2160", 1080: "1080", 1920: "1080",
		720: "720", 730: "720", 480: "sd", 0: "sd",
	}
	for in, want := range cases {
		if got := qualityFromInt(in); got != want {
			t.Fatalf("qualityFromInt(%d) = %q, want %q", in, got, want)
		}
	}
}
