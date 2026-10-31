package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type SummaryState struct {
	Date     string   `json:"date"`      // YYYY-MM-DD
	SentKeys []string `json:"sent_keys"` // รายการคีย์สแกน (YYYYMMDDHHMMSS_enroll) ที่ส่งสรุปไปแล้วในวันนี้
	Updated  string   `json:"updated"`
}

var (
	summaryStateMu   sync.Mutex
	summaryStatePath = filepath.Join("data", "summary_state.json")
)

// BuildScanKeys แปลง map การสแกนทั้งหมดของวันให้เป็นรายการคีย์ที่ไม่ซ้ำและเรียงลำดับแล้ว
func BuildScanKeys(scans map[int][]time.Time) []string {
	seen := make(map[string]bool)
	var keys []string
	for enroll, times := range scans {
		for _, t := range times {
			k := fmt.Sprintf("%s_%d", t.Format("20060102150405"), enroll)
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

// HasNewSummaryScans ตรวจสอบว่ามีข้อมูลการเช็คชื่อ/สแกนนิ้วใหม่เพิ่มขึ้นจากรอบส่งสรุปครั้งล่าสุดของวันนี้หรือไม่
// คืนค่า false หากวันนี้ยังไม่มีการสแกนเลย (0 รายการ) หรือไม่มีรายการสแกนใหม่เพิ่มจากรอบก่อนหน้า
func HasNewSummaryScans(day time.Time, scans map[int][]time.Time) bool {
	currentKeys := BuildScanKeys(scans)
	if len(currentKeys) == 0 {
		return false
	}

	summaryStateMu.Lock()
	defer summaryStateMu.Unlock()

	dayStr := day.Format("2006-01-02")
	st := readSummaryStateUnsafe()
	if st.Date != dayStr {
		return true
	}

	prevSet := make(map[string]bool, len(st.SentKeys))
	for _, k := range st.SentKeys {
		prevSet[k] = true
	}

	for _, k := range currentKeys {
		if !prevSet[k] {
			return true
		}
	}
	return false
}

// MarkSummaryScansSent บันทึกชุดการสแกนที่ส่งรายงานสรุปสำเร็จแล้วของวันนี้
func MarkSummaryScansSent(day time.Time, scans map[int][]time.Time) {
	currentKeys := BuildScanKeys(scans)

	summaryStateMu.Lock()
	defer summaryStateMu.Unlock()

	dayStr := day.Format("2006-01-02")
	st := SummaryState{
		Date:     dayStr,
		SentKeys: currentKeys,
		Updated:  time.Now().Format("2006-01-02 15:04:05"),
	}
	_ = writeSummaryStateUnsafe(st)
}

func readSummaryStateUnsafe() SummaryState {
	b, err := os.ReadFile(summaryStatePath)
	if err != nil {
		return SummaryState{}
	}
	var st SummaryState
	if err := json.Unmarshal(b, &st); err != nil {
		return SummaryState{}
	}
	return st
}

func writeSummaryStateUnsafe(st SummaryState) error {
	_ = os.MkdirAll(filepath.Dir(summaryStatePath), 0755)
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := summaryStatePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, summaryStatePath)
}
