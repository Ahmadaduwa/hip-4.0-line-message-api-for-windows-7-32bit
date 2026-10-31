package telegram

import (
	"os"
	"strings"
	"testing"
	"time"

	"stw_teacher_check/attendance"
)

func TestFormatEventsBatchingAndExactSeconds(t *testing.T) {
	t1 := time.Date(2026, 10, 23, 7, 12, 34, 0, time.Local)
	t2 := time.Date(2026, 10, 23, 7, 45, 10, 0, time.Local)
	ev1 := ScanEvent{Name: "ครูสมชาย", ScanTime: t1, Status: "present"}
	ev2 := ScanEvent{Name: "ครูสมศรี", ScanTime: t2, Status: "late", LateMins: 15}

	// <= 5: แยกข้อความ พร้อมแสดงวินาทีจริงจาก DB
	msgs := FormatEvents([]ScanEvent{ev1, ev2})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "07:12:34 น.") || !strings.Contains(msgs[0], "ตรงเวลา") {
		t.Errorf("unexpected message[0] format: %s", msgs[0])
	}
	if !strings.Contains(msgs[1], "07:45:10 น.") || !strings.Contains(msgs[1], "สาย 15 นาที") {
		t.Errorf("unexpected message[1] format: %s", msgs[1])
	}

	// > 5: รวม 1 ข้อความ
	var batch []ScanEvent
	for i := 0; i < 6; i++ {
		batch = append(batch, ScanEvent{
			Name:      "User",
			ScanTime:  t1.Add(time.Duration(i) * time.Minute),
			Status:    "present",
			ScanIndex: 1,
		})
	}
	batchMsgs := FormatEvents(batch)
	if len(batchMsgs) != 1 {
		t.Fatalf("expected 1 merged message for batch > 5, got %d", len(batchMsgs))
	}
	if !strings.Contains(batchMsgs[0], "รายการสแกนนิ้วล่าสุด (6 รายการ)") {
		t.Errorf("unexpected batch header: %s", batchMsgs[0])
	}
}

func TestDiffScansDeduplicationAndTeacherFallback(t *testing.T) {
	tmpFile := "data/test_sentstate.json"
	_ = os.Remove(tmpFile)
	defer os.Remove(tmpFile)

	sentStore := NewSentStore(tmpFile)
	now := time.Date(2026, 10, 23, 7, 10, 25, 0, time.Local)

	people := []attendance.Person{
		{Enroll: 1, Name: "นาย ก"},
	}
	// สแกนซ้ำภายใน 1 นาทีของ Enroll 1 และสแกนของ Enroll 99 ที่ไม่มีชื่อในตาราง Student
	scans := map[int][]time.Time{
		1:  {now, now.Add(1 * time.Minute), now.Add(10 * time.Minute)},
		99: {now.Add(2 * time.Minute)},
	}

	summary := attendance.Summary{
		Results: []attendance.Result{
			{Person: people[0], Status: "present"},
		},
	}

	events := DiffScans(people, scans, summary, sentStore, 2*time.Minute)
	// ควรได้ 3 events: Enroll 1 (07:10:25), Enroll 99 (07:12:25), Enroll 1 (07:20:25)
	if len(events) != 3 {
		t.Fatalf("expected 3 events after dedupe, got %d", len(events))
	}
	if events[1].Name != "รหัส 99" {
		t.Errorf("expected fallback name 'รหัส 99', got %q", events[1].Name)
	}
	if key := makeScanKey(99, now); key != "20261023071025_99" {
		t.Errorf("unexpected makeScanKey: %s", key)
	}
}
