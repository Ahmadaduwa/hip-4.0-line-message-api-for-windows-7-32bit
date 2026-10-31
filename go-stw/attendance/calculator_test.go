package attendance

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseHM(t *testing.T) {
	tests := []struct {
		input string
		expH  int
		expM  int
		expOk bool
	}{
		{"07:30", 7, 30, true},
		{"7:30", 7, 30, true},
		{"07.30", 7, 30, true},
		{"09:00", 9, 0, true},
		{"23:59", 23, 59, true},
		{"24:00", 0, 0, false},
		{"invalid", 0, 0, false},
		{"", 0, 0, false},
	}

	for _, tt := range tests {
		h, m, ok := ParseHM(tt.input)
		if ok != tt.expOk || (ok && (h != tt.expH || m != tt.expM)) {
			t.Errorf("ParseHM(%q) = (%d, %d, %v), expected (%d, %d, %v)", tt.input, h, m, ok, tt.expH, tt.expM, tt.expOk)
		}
	}
}

func TestThaiDate(t *testing.T) {
	d := time.Date(2026, 10, 23, 8, 0, 0, 0, time.Local)
	s := ThaiDate(d)
	// 2026 + 543 = 2569
	if !strings.Contains(s, "2569") || !strings.Contains(s, "ตุลาคม") || !strings.Contains(s, "วันศุกร์") {
		t.Errorf("ThaiDate(%v) = %q, missing expected Thai date parts", d, s)
	}
}

func TestComputeAndBuildMessage(t *testing.T) {
	day := time.Date(2026, 10, 23, 0, 0, 0, 0, time.Local)
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.Local)

	people := []Person{
		{Enroll: 1, Code: "001", Name: "นายสมชาย", ShiftCode: "01"},
		{Enroll: 2, Code: "002", Name: "นางสาวสมศรี", ShiftCode: "01"},
		{Enroll: 3, Code: "003", Name: "นายมานะ", ShiftCode: "01"},
		{Enroll: 4, Code: "004", Name: "นายชูใจ", ShiftCode: "01"},
	}

	scans := map[int][]time.Time{
		1: {time.Date(2026, 10, 23, 7, 15, 0, 0, time.Local)}, // On-time (before 07:30)
		2: {time.Date(2026, 10, 23, 7, 45, 0, 0, time.Local)}, // Late (after 07:30 by 15 mins)
	}

	shifts := map[string]Shift{
		"01": {Code: "01", Name: "กะปกติ"},
	}

	leaves := map[string]string{
		"003": "ลาป่วย",
	}

	holidays := map[string]string{}
	schoolWorkdays := [7]int{0, 1, 1, 1, 1, 1, 0} // Mon-Fri

	summary := Compute(
		day,
		people,
		scans,
		shifts,
		map[int]string{},
		leaves,
		holidays,
		schoolWorkdays,
		"07:30",
		now,
	)

	if !summary.IsWorkday {
		t.Fatalf("expected Friday to be a workday")
	}

	pList := summary.ByStatus("present")
	if len(pList) != 1 || pList[0].Person.Enroll != 1 {
		t.Errorf("expected 1 present (enroll 1), got %v", pList)
	}

	lList := summary.ByStatus("late")
	if len(lList) != 1 || lList[0].Person.Enroll != 2 || lList[0].LateMinutes != 15 {
		t.Errorf("expected 1 late (enroll 2) with 15 late mins, got %v", lList)
	}

	leaveList := summary.ByStatus("leave")
	if len(leaveList) != 1 || leaveList[0].Person.Enroll != 3 || leaveList[0].LeaveName != "ลาป่วย" {
		t.Errorf("expected 1 leave (enroll 3), got %v", leaveList)
	}

	abList := summary.ByStatus("absent")
	if len(abList) != 1 || abList[0].Person.Enroll != 4 {
		t.Errorf("expected 1 absent (enroll 4), got %v", abList)
	}

	msg := BuildMessage(summary, "STW_TeacherCheck", &now, true)
	if !strings.Contains(msg, "สรุปการมาทำงาน STW_TeacherCheck") {
		t.Errorf("msg header missing, got: %s", msg)
	}
	if !strings.Contains(msg, "⏰ มาสาย (1 คน):") || !strings.Contains(msg, "สาย 15 นาที") {
		t.Errorf("msg late section incorrect, got: %s", msg)
	}
	if !strings.Contains(msg, "📝 ลางาน (1 คน):") || !strings.Contains(msg, "ลาป่วย") {
		t.Errorf("msg leave section incorrect, got: %s", msg)
	}
}

func TestDistributeBudgets(t *testing.T) {
	// Case 1: Small requests should get 100% allocation
	b1 := DistributeBudgets([]int{5, 5, 5, 0}, 100)
	if b1[0] != 5 || b1[1] != 5 || b1[2] != 5 || b1[3] != 0 {
		t.Errorf("Case 1 failed, got: %v", b1)
	}

	// Case 2: Uneven requests with large lists
	// late: 2, leave: 1, absent: 80, present: 120, total budget: 100
	// 2 + 1 = 3 allocated to small lists. Remaining 97 split roughly evenly (48 and 49)
	b2 := DistributeBudgets([]int{2, 1, 80, 120}, 100)
	if b2[0] != 2 || b2[1] != 1 {
		t.Errorf("Case 2 small lists should be full, got: %v", b2)
	}
	if b2[2] < 45 || b2[3] < 45 {
		t.Errorf("Case 2 large lists should receive roughly equal fair share, got: %v", b2)
	}
	if b2[0]+b2[1]+b2[2]+b2[3] != 100 {
		t.Errorf("Case 2 total budget should sum to 100, got: %d", b2[0]+b2[1]+b2[2]+b2[3])
	}
}

func TestPrioritizedBuildMessage(t *testing.T) {
	day := time.Date(2026, 10, 23, 0, 0, 0, 0, time.Local)
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.Local)

	// Create 10 late, 5 leave, 25 absent, and 120 present
	var results []Result
	for i := 1; i <= 10; i++ {
		tScan := time.Date(2026, 10, 23, 7, 45, 0, 0, time.Local)
		results = append(results, Result{
			Person:      Person{Enroll: i, Code: "L", Name: "สาย"},
			Status:      "late",
			FirstScan:   &tScan,
			LateMinutes: 15,
		})
	}
	for i := 11; i <= 15; i++ {
		results = append(results, Result{
			Person:    Person{Enroll: i, Code: "V", Name: "ลา"},
			Status:    "leave",
			LeaveName: "ลากิจ",
		})
	}
	for i := 16; i <= 40; i++ {
		results = append(results, Result{
			Person: Person{Enroll: i, Code: "A", Name: "ขาด"},
			Status: "absent",
		})
	}
	for i := 41; i <= 160; i++ {
		tScan := time.Date(2026, 10, 23, 7, 0, 0, 0, time.Local)
		results = append(results, Result{
			Person:    Person{Enroll: i, Code: "P", Name: "ปกติ"},
			Status:    "present",
			FirstScan: &tScan,
		})
	}

	summary := Summary{
		Day:       day,
		IsWorkday: true,
		Results:   results,
	}

	msg := BuildMessage(summary, "โรงเรียนแสงธรรมวิทยา", &now, true)

	// All 10 late, 5 leave, and 25 absent (total 40) must be shown without truncation
	if !strings.Contains(msg, "มาสาย (10 คน):") {
		t.Errorf("missing late header in msg: %s", msg)
	}
	if !strings.Contains(msg, "ลางาน (5 คน):") {
		t.Errorf("missing leave header in msg: %s", msg)
	}
	if !strings.Contains(msg, "ยังไม่พบการสแกน/ขาด (25 คน):") {
		t.Errorf("missing absent header in msg: %s", msg)
	}
	// Present had 120 people, total max budget 100, 40 used by high priority -> 60 allocated to present
	if !strings.Contains(msg, "มาตรงเวลา (120 คน):") {
		t.Errorf("missing present header in msg: %s", msg)
	}
	// 120 - 60 = 60 remaining
	if !strings.Contains(msg, "และคนอื่นๆ อีก 60 คน") {
		t.Errorf("expected present to be truncated to remaining budget of 60, msg: %s", msg)
	}

	// Verify exact section order: ขาด -> ลา -> สาย -> ปกติ
	idxAbsent := strings.Index(msg, "ยังไม่พบการสแกน/ขาด (25 คน):")
	idxLeave := strings.Index(msg, "ลางาน (5 คน):")
	idxLate := strings.Index(msg, "มาสาย (10 คน):")
	idxPresent := strings.Index(msg, "มาตรงเวลา (120 คน):")

	if !(idxAbsent < idxLeave && idxLeave < idxLate && idxLate < idxPresent) {
		t.Errorf("expected order: ขาด < ลา < สาย < ปกติ, got indices: absent=%d, leave=%d, late=%d, present=%d",
			idxAbsent, idxLeave, idxLate, idxPresent)
	}
}

func TestUnifiedStandardReportOnWeekend(t *testing.T) {
	// วันอาทิตย์ที่ 25 ตุลาคม 2026 (เดิมเป็นวันหยุดประจำสัปดาห์)
	sunday := time.Date(2026, 10, 25, 0, 0, 0, 0, time.Local)
	now := time.Date(2026, 10, 25, 9, 0, 0, 0, time.Local)

	people := []Person{
		{Enroll: 1, Code: "001", Name: "ครู ก", ShiftCode: "01"},
		{Enroll: 2, Code: "002", Name: "ครู ข", ShiftCode: "01"},
		{Enroll: 3, Code: "003", Name: "ครู ค", ShiftCode: "01"},
	}
	scans := map[int][]time.Time{
		1: {time.Date(2026, 10, 25, 7, 10, 0, 0, time.Local)}, // ปกติ
		2: {time.Date(2026, 10, 25, 7, 50, 0, 0, time.Local)}, // สาย 20 นาที
	}

	summary := Compute(
		sunday,
		people,
		scans,
		map[string]Shift{"01": {Code: "01", Name: "กะปกติ"}},
		nil,
		nil,
		map[string]string{"2026-10-25": "วันหยุดพิเศษ"},
		[7]int{0, 1, 1, 1, 1, 1, 0},
		"07:30",
		now,
	)

	if len(summary.ByStatus("present")) != 1 || len(summary.ByStatus("late")) != 1 || len(summary.ByStatus("absent")) != 1 {
		t.Fatalf("expected standard calculation on Sunday: present=1, late=1, absent=1, got %+v", summary.Results)
	}

	msg := BuildMessage(summary, "โรงเรียนแสงธรรมวิทยา", &now, true)
	if strings.Contains(msg, "วันนี้เป็นวันหยุด") {
		t.Errorf("should not render holiday mode message, got: %s", msg)
	}
	if !strings.Contains(msg, "มาสาย (1 คน):") || !strings.Contains(msg, "มาตรงเวลา (1 คน):") {
		t.Errorf("missing standard report sections, got: %s", msg)
	}
}

func TestBuildFullMessageNoTruncation(t *testing.T) {
	day := time.Date(2026, 10, 23, 0, 0, 0, 0, time.Local)
	now := time.Date(2026, 10, 23, 9, 0, 0, 0, time.Local)
	scanTime := time.Date(2026, 10, 23, 7, 10, 0, 0, time.Local)

	var results []Result
	for i := 1; i <= 130; i++ {
		st := scanTime
		results = append(results, Result{
			Person:    Person{Enroll: i, Name: fmt.Sprintf("ครูคนที่_%03d", i)},
			Status:    "present",
			FirstScan: &st,
		})
	}

	summary := Summary{
		Day:        day,
		Results:    results,
		TotalScans: 130,
	}

	// LINE: BuildMessage จำกัด 100 คน -> ต้องมี "... และคนอื่นๆ อีก 30 คน"
	lineMsg := BuildMessage(summary, "โรงเรียนแสงธรรมวิทยา", &now, true)
	if !strings.Contains(lineMsg, "... และคนอื่นๆ อีก 30 คน") {
		t.Errorf("expected LINE BuildMessage to truncate at 100 people, got: %s", lineMsg)
	}

	// Telegram: BuildFullMessage แสดงครบ 130 คน -> ต้องไม่มี "... และคนอื่นๆ อีก" และมีชื่อคนที่ 130
	tgMsg := BuildFullMessage(summary, "โรงเรียนแสงธรรมวิทยา", &now, true)
	if strings.Contains(tgMsg, "... และคนอื่นๆ อีก") {
		t.Errorf("expected Telegram BuildFullMessage to list all people without truncation")
	}
	if !strings.Contains(tgMsg, "ครูคนที่_130") {
		t.Errorf("expected Telegram BuildFullMessage to contain person 130")
	}
}



