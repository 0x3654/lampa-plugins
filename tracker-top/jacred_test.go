package main

import (
	"encoding/xml"
	"os"
	"testing"
)

// TestJacredParseFixture — реальный ответ newznab (снят с ae2-инстанса
// 22.10): гб title, torznab:attr с magnet/сидерами/годом/сайтом
func TestJacredParseFixture(t *testing.T) {
	raw, err := os.ReadFile("tests/jr-sample.xml")
	if err != nil {
		t.Skip("нет фикстуры tests/jr-sample.xml")
	}

	var feed torznabFeed
	if err := xml.Unmarshal(raw, &feed); err != nil {
		t.Fatalf("разбор: %v", err)
	}

	if len(feed.Items) == 0 {
		t.Fatal("0 item'ов в живом ответе")
	}

	first := feed.Items[0]
	if first.Title == "" {
		t.Fatal("пустой title")
	}

	var magnet, site string
	var seeders int
	for _, a := range first.Attrs {
		switch a.Name {
		case "magneturl":
			magnet = a.Value
		case "site":
			site = a.Value
		case "seeders":
			seeders = atoi(a.Value)
		}
	}

	if magnet == "" || site == "" || seeders == 0 {
		t.Fatalf("атрибуты не прочитались: magnet=%q site=%q seeders=%d", magnet, site, seeders)
	}

	// полный путь: title → Item (Ru/Orig/Year/Quality/Voice)
	it := jacredItemFromTitle(first.Title, first.Attrs)
	if it.Ru == "" || it.Year == 0 {
		t.Fatalf("parseTitle не разобрал: ru=%q year=%d", it.Ru, it.Year)
	}
	if it.Magnet == "" || it.Source == "jacred" {
		t.Fatalf("атрибуты не перенесены: magnet=%q source=%q", it.Magnet, it.Source)
	}
}
