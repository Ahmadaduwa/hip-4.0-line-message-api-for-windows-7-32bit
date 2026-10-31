package attendance

import (
	"strings"
	"testing"
	"time"
)

func TestBuildMessageSortsByScanTime(t *testing.T) {
	day := time.Date(2026, 10, 23, 0, 0, 0, 0, time.Local)
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.Local)
	at := func(h, m int) time.Time { return time.Date(2026, 10, 23, h, m, 0, 0, time.Local) }

	// ลำดับใน DB สลับกับลำดับเวลาสแกน
	people := []Person{
		{Enroll: 1, Code: "1", Name: "ค-ปกติ-0725"},
		{Enroll: 2, Code: "2", Name: "ก-ปกติ-0705"},
		{Enroll: 3, Code: "3", Name: "ข-ปกติ-0715"},
		{Enroll: 4, Code: "4", Name: "จ-สาย-0810"},
		{Enroll: 5, Code: "5", Name: "ง-สาย-0740"},
		{Enroll: 6, Code: "6", Name: "ฮ-ขาด"},
		{Enroll: 7, Code: "7", Name: "ก-ขาด"},
	}
	scans := map[int][]time.Time{
		1: {at(7, 25)}, 2: {at(7, 5)}, 3: {at(7, 15), at(16, 30)},
		4: {at(8, 10)}, 5: {at(7, 40)},
	}
	s := Compute(day, people, scans, map[string]Shift{}, nil, nil, nil,
		[7]int{0, 1, 1, 1, 1, 1, 0}, "07:30", now)
	msg := BuildMessage(s, "X", &now, true)

	assertOrder(t, msg, "ก-ปกติ-0705", "ข-ปกติ-0715", "ค-ปกติ-0725")
	assertOrder(t, msg, "ง-สาย-0740", "จ-สาย-0810")
	assertOrder(t, msg, "ก-ขาด", "ฮ-ขาด")
}

func TestSortByFirstScanNilLast(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 7, 0, 0, 0, time.Local)
	list := []Result{
		{Person: Person{Name: "b"}},
		{Person: Person{Name: "a"}, FirstScan: &t1},
	}
	SortByFirstScan(list)
	if list[0].Person.Name != "a" {
		t.Fatalf("expected scanned first, got %v", list)
	}
}

func assertOrder(t *testing.T, msg string, names ...string) {
	t.Helper()
	prev := -1
	for _, n := range names {
		idx := strings.Index(msg, n)
		if idx < 0 || idx < prev {
			t.Fatalf("expected order %v, got msg:\n%s", names, msg)
		}
		prev = idx
	}
}

