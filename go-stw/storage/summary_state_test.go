package storage

import (
	"os"
	"testing"
	"time"
)

func TestDynamicSummaryState(t *testing.T) {
	origPath := summaryStatePath
	summaryStatePath = "data/test_summary_state.json"
	_ = os.Remove(summaryStatePath)
	defer func() {
		_ = os.Remove(summaryStatePath)
		summaryStatePath = origPath
	}()

	day := time.Date(2026, 10, 31, 0, 0, 0, 0, time.Local)
	t1 := time.Date(2026, 10, 31, 7, 15, 0, 0, time.Local)
	t2 := time.Date(2026, 10, 31, 7, 45, 0, 0, time.Local)
	t3 := time.Date(2026, 10, 31, 16, 30, 0, 0, time.Local)

	// 1. ไม่มีใครสแกนเลย (เช่น วันหยุดนักขัตฤกษ์) -> ต้องไม่ส่ง (false)
	emptyScans := map[int][]time.Time{}
	if HasNewSummaryScans(day, emptyScans) {
		t.Fatal("expected false when there are 0 scans")
	}

	// 2. รอบ 09:00 มีคนสแกน 2 คน -> ต้องส่ง (true)
	morningScans := map[int][]time.Time{
		1: {t1},
		2: {t2},
	}
	if !HasNewSummaryScans(day, morningScans) {
		t.Fatal("expected true for first round with scans")
	}
	MarkSummaryScansSent(day, morningScans)

	// 3. รอบ 18:00 ถ้าไม่มีใครสแกนเพิ่มเลย -> ต้องไม่ส่งซ้ำ (false)
	if HasNewSummaryScans(day, morningScans) {
		t.Fatal("expected false when no new scans were added since 09:00")
	}

	// 4. รอบ 18:00 มีการสแกนเพิ่ม (เช่น สแกนออก 16:30) -> ต้องส่ง (true)
	eveningScans := map[int][]time.Time{
		1: {t1, t3},
		2: {t2},
	}
	if !HasNewSummaryScans(day, eveningScans) {
		t.Fatal("expected true when new scan arrived before 18:00")
	}
	MarkSummaryScansSent(day, eveningScans)

	// 5. ข้ามวันใหม่ -> ต้องรีเซ็ตและส่งได้เมื่อมีสแกนของวันใหม่
	nextDay := day.AddDate(0, 0, 1)
	nextDayScans := map[int][]time.Time{
		1: {t1.AddDate(0, 0, 1)},
	}
	if !HasNewSummaryScans(nextDay, nextDayScans) {
		t.Fatal("expected true on new day")
	}
}
