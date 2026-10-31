package attendance

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Person struct {
	Enroll    int    `json:"enroll"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Level     string `json:"level"`
	ShiftCode string `json:"shift_code"`
}

type Shift struct {
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Start     *time.Time `json:"start"`
	End       *time.Time `json:"end"`
	IsDayOff  bool       `json:"is_dayoff"`
}

type Result struct {
	Person      Person     `json:"person"`
	Status      string     `json:"status"` // present, late, absent, leave, dayoff, pending
	FirstScan   *time.Time `json:"first_scan,omitempty"`
	LastScan    *time.Time `json:"last_scan,omitempty"`
	LateMinutes int        `json:"late_minutes"`
	Shift       *Shift     `json:"shift,omitempty"`
	LeaveName   string     `json:"leave_name,omitempty"`
}

type Summary struct {
	Day        time.Time `json:"day"`
	IsWorkday  bool      `json:"is_workday"`
	Holiday    string    `json:"holiday,omitempty"`
	Results    []Result  `json:"results"`
	TotalScans int       `json:"total_scans"`
}

func (s *Summary) ByStatus(status string) []Result {
	var res []Result
	for _, r := range s.Results {
		if r.Status == status {
			res = append(res, r)
		}
	}
	return res
}

func ParseHM(s string) (int, int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	re := regexp.MustCompile(`^(\d{1,2})[:.](\d{2})`)
	m := re.FindStringSubmatch(s)
	if len(m) < 3 {
		return 0, 0, false
	}
	h, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	if h >= 0 && h < 24 && mi >= 0 && mi < 60 {
		return h, mi, true
	}
	return 0, 0, false
}

func NextRunTime(times []string, now time.Time) time.Time {
	var candidates []time.Time
	for _, tStr := range times {
		h, m, ok := ParseHM(tStr)
		if !ok {
			continue
		}
		tToday := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		if tToday.After(now) {
			candidates = append(candidates, tToday)
		}
		tTomorrow := tToday.AddDate(0, 0, 1)
		candidates = append(candidates, tTomorrow)
	}

	if len(candidates) == 0 {
		return time.Date(now.Year(), now.Month(), now.Day()+1, 9, 5, 0, 0, now.Location())
	}

	minT := candidates[0]
	for _, c := range candidates[1:] {
		if c.Before(minT) {
			minT = c
		}
	}
	return minT
}

func FormatNextRun(nextT time.Time, now time.Time) string {
	if nextT.Day() == now.Day() {
		return nextT.Format("วันนี้ 15:04 น.")
	}
	return nextT.Format("พรุ่งนี้ 15:04 น.")
}

func Compute(
	day time.Time,
	people []Person,
	scans map[int][]time.Time,
	shifts map[string]Shift,
	dailyShift map[int]string,
	leaves map[string]string,
	holidays map[string]string,
	schoolWorkdays [7]int,
	lateCutoffStr string,
	now time.Time,
) Summary {
	dateStr := day.Format("2006-01-02")
	// ใช้รูปแบบวันทำงานมาตรฐานเดียวกันทุกวัน (ไม่แยกโหมดวันหยุด เพราะใช้ Dynamic Summary ตัดสินจากข้อมูลสแกนจริง)
	holidayReason := holidays[dateStr]
	isWorkday := true

	// Late Cutoff Time (Default 07:30)
	lateH, lateM, ok := ParseHM(lateCutoffStr)
	if !ok {
		lateH, lateM = 7, 30
	}
	lateCutoff := time.Date(day.Year(), day.Month(), day.Day(), lateH, lateM, 0, 0, day.Location())

	totalScans := 0
	for _, times := range scans {
		totalScans += len(times)
	}

	summary := Summary{
		Day:        day,
		IsWorkday:  isWorkday,
		Holiday:    holidayReason,
		Results:    make([]Result, 0, len(people)),
		TotalScans: totalScans,
	}

	for _, p := range people {
		times := scans[p.Enroll]
		sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

		shiftCode := dailyShift[p.Enroll]
		if shiftCode == "" {
			shiftCode = p.ShiftCode
		}
		sh, hasShift := shifts[shiftCode]
		if !hasShift {
			sh = Shift{Code: "01", Name: "กะปกติ"}
		}

		r := Result{
			Person: p,
			Shift:  &sh,
		}
		if len(times) > 0 {
			r.FirstScan = &times[0]
			if len(times) > 1 {
				r.LastScan = &times[len(times)-1]
			}
		}

		if leaveName, hasLeave := leaves[p.Code]; hasLeave {
			r.Status = "leave"
			r.LeaveName = leaveName
		} else if len(times) > 0 {
			r.Status = "present"
			// ตรวจสอบว่าสายหรือไม่ (หลัง lateCutoff เช่น 07:30)
			if r.FirstScan.After(lateCutoff) {
				r.Status = "late"
				diff := r.FirstScan.Sub(lateCutoff)
				r.LateMinutes = int(diff.Minutes())
			}
		} else if day.Format("2006-01-02") == now.Format("2006-01-02") && now.Before(lateCutoff) {
			r.Status = "pending"
		} else {
			r.Status = "absent"
		}

		summary.Results = append(summary.Results, r)
	}

	return summary
}

// ================== THAI FORMATTING ==================

var thaiDays = []string{"วันอาทิตย์", "วันจันทร์", "วันอังคาร", "วันพุธ", "วันพฤหัสบดี", "วันศุกร์", "วันเสาร์"}
var thaiMonths = []string{"", "มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน", "กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม"}

func ThaiDate(d time.Time) string {
	dayName := thaiDays[d.Weekday()]
	monthName := thaiMonths[d.Month()]
	yearThai := d.Year() + 543
	return fmt.Sprintf("%sที่ %d %s %d", dayName, d.Day(), monthName, yearThai)
}

func DistributeBudgets(requests []int, totalBudget int) []int {
	n := len(requests)
	allocated := make([]int, n)
	if n == 0 || totalBudget <= 0 {
		return allocated
	}

	remainingBudget := totalBudget
	activeIndices := make(map[int]bool)
	for i, req := range requests {
		if req > 0 {
			activeIndices[i] = true
		}
	}

	for len(activeIndices) > 0 && remainingBudget > 0 {
		share := remainingBudget / len(activeIndices)
		if share == 0 {
			share = 1
		}
		progress := false
		for idx := range activeIndices {
			req := requests[idx] - allocated[idx]
			if req <= 0 {
				delete(activeIndices, idx)
				continue
			}
			add := share
			if add > req {
				add = req
			}
			if add > remainingBudget {
				add = remainingBudget
			}
			allocated[idx] += add
			remainingBudget -= add
			progress = true
			if allocated[idx] >= requests[idx] {
				delete(activeIndices, idx)
			}
			if remainingBudget <= 0 {
				break
			}
		}
		if !progress {
			break
		}
	}
	return allocated
}

func appendPersonList(sb *strings.Builder, headerIcon, headerTitle string, list []Result, maxCount int, formatSub func(r Result) string) {
	if len(list) == 0 || maxCount <= 0 {
		return
	}
	sb.WriteString(fmt.Sprintf("\n%s %s (%d คน):\n", headerIcon, headerTitle, len(list)))
	showCount := len(list)
	truncated := false
	if showCount > maxCount {
		showCount = maxCount
		truncated = true
	}
	for i := 0; i < showCount; i++ {
		r := list[i]
		sub := formatSub(r)
		sb.WriteString(fmt.Sprintf(" • %s%s\n", r.Person.Name, sub))
	}
	if truncated {
		rem := len(list) - showCount
		sb.WriteString(fmt.Sprintf(" • ... และคนอื่นๆ อีก %d คน (รวมทั้งหมด %d คน)\n", rem, len(list)))
	}
}

// SortByFirstScan เรียงรายชื่อตามเวลาสแกนแรกจากเช้าไปสาย (ไม่มีเวลาสแกนไว้ท้าย, เวลาเท่ากันเรียงตามชื่อ)
func SortByFirstScan(list []Result) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i].FirstScan, list[j].FirstScan
		switch {
		case a == nil && b == nil:
			return list[i].Person.Name < list[j].Person.Name
		case a == nil:
			return false
		case b == nil:
			return true
		case a.Equal(*b):
			return list[i].Person.Name < list[j].Person.Name
		}
		return a.Before(*b)
	})
}

// SortByName เรียงรายชื่อตามชื่อ
func SortByName(list []Result) {
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Person.Name < list[j].Person.Name
	})
}

// BuildMessage สร้างข้อความสรุปสำหรับ LINE (จำกัดโควตาสูงสุด 100 รายชื่อเพื่อไม่ให้เกิน 5,000 ตัวอักษรและโควตาข้อความของ LINE)
func BuildMessage(summary Summary, company string, now *time.Time, showPresent bool) string {
	return BuildMessageWithLimit(summary, company, now, showPresent, 100)
}

// BuildFullMessage สร้างข้อความสรุปแบบแสดงรายชื่อครบ 100% ทุกคน (สำหรับ Telegram ที่ไม่มีเพดานโควตาข้อความและมีระบบตัดแบ่งข้อความอัตโนมัติ)
func BuildFullMessage(summary Summary, company string, now *time.Time, showPresent bool) string {
	return BuildMessageWithLimit(summary, company, now, showPresent, 0)
}

func BuildMessageWithLimit(summary Summary, company string, now *time.Time, showPresent bool, totalMaxBudget int) string {
	presentList := summary.ByStatus("present")
	lateList := summary.ByStatus("late")
	absentList := summary.ByStatus("absent")
	leaveList := summary.ByStatus("leave")
	pendingList := summary.ByStatus("pending")

	// สาย/ปกติ เรียงตามเวลาสแกนจริง, ขาด/ลา เรียงตามชื่อ
	SortByFirstScan(presentList)
	SortByFirstScan(lateList)
	SortByName(absentList)
	SortByName(leaveList)

	dateHeader := ThaiDate(summary.Day)
	if now != nil {
		dateHeader += fmt.Sprintf(" (ข้อมูล ณ %s)", now.Format("15:04 น."))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 สรุปการมาทำงาน %s\n%s\n\n", company, dateHeader))

	total := len(summary.Results)
	checkedIn := len(presentList) + len(lateList)
	sb.WriteString(fmt.Sprintf("👥 ทั้งหมด: %d คน | เข้างานแล้ว: %d คน\n", total, checkedIn))
	sb.WriteString(fmt.Sprintf("❌ ขาด: %d   |  📝 ลา: %d\n", len(absentList), len(leaveList)))
	sb.WriteString(fmt.Sprintf("⏰ สาย: %d   |  ✅ ปกติ: %d\n", len(lateList), len(presentList)))
	if len(pendingList) > 0 {
		sb.WriteString(fmt.Sprintf("⏳ ยังไม่ถึงเวลา: %d คน\n", len(pendingList)))
	}
	sb.WriteString("------------------------------\n")

	reqAbsent := len(absentList)
	reqLeave := len(leaveList)
	reqLate := len(lateList)
	reqPresent := len(presentList)

	budgetAbsent := reqAbsent
	budgetLeave := reqLeave
	budgetLate := reqLate
	budgetPresent := 0
	if showPresent {
		budgetPresent = reqPresent
	}

	// หากระบุ totalMaxBudget > 0 (เช่น 100 คนสำหรับ LINE) ให้จัดสรรโควตาตามลำดับความสำคัญ: ขาด -> ลา -> สาย -> ปกติ
	if totalMaxBudget > 0 {
		minAbsent := reqAbsent
		if minAbsent > 3 {
			minAbsent = 3
		}
		minLeave := reqLeave
		if minLeave > 3 {
			minLeave = 3
		}
		minLate := reqLate
		if minLate > 3 {
			minLate = 3
		}
		minPresent := 0
		if showPresent {
			minPresent = reqPresent
			if minPresent > 3 {
				minPresent = 3
			}
		}

		budgetAbsent = minAbsent
		budgetLeave = minLeave
		budgetLate = minLate
		budgetPresent = minPresent

		remBudget := totalMaxBudget - (budgetAbsent + budgetLeave + budgetLate + budgetPresent)
		if remBudget < 0 {
			remBudget = 0
		}

		if remBudget > 0 && budgetAbsent < reqAbsent {
			add := reqAbsent - budgetAbsent
			if add > remBudget {
				add = remBudget
			}
			budgetAbsent += add
			remBudget -= add
		}

		if remBudget > 0 && budgetLeave < reqLeave {
			add := reqLeave - budgetLeave
			if add > remBudget {
				add = remBudget
			}
			budgetLeave += add
			remBudget -= add
		}

		if remBudget > 0 && budgetLate < reqLate {
			add := reqLate - budgetLate
			if add > remBudget {
				add = remBudget
			}
			budgetLate += add
			remBudget -= add
		}

		if remBudget > 0 && showPresent && budgetPresent < reqPresent {
			add := reqPresent - budgetPresent
			if add > remBudget {
				add = remBudget
			}
			budgetPresent += add
			remBudget -= add
		}
	}

	// 1. แสดงรายชื่อ: ขาด
	appendPersonList(&sb, "❌", "ยังไม่พบการสแกน/ขาด", absentList, budgetAbsent, func(r Result) string {
		return ""
	})

	// 2. แสดงรายชื่อ: ลา
	appendPersonList(&sb, "📝", "ลางาน", leaveList, budgetLeave, func(r Result) string {
		return fmt.Sprintf(" (%s)", r.LeaveName)
	})

	// 3. แสดงรายชื่อ: สาย
	appendPersonList(&sb, "⏰", "มาสาย", lateList, budgetLate, func(r Result) string {
		if r.FirstScan != nil {
			return fmt.Sprintf(" [เข้า %s, สาย %d นาที]", r.FirstScan.Format("15:04"), r.LateMinutes)
		}
		return ""
	})

	// 4. แสดงรายชื่อ: ปกติ (ถ้าเปิด showPresent และยังมีโควตาเหลือ)
	if showPresent && budgetPresent > 0 {
		appendPersonList(&sb, "✅", "มาตรงเวลา", presentList, budgetPresent, func(r Result) string {
			if r.FirstScan != nil {
				return fmt.Sprintf(" (%s)", r.FirstScan.Format("15:04"))
			}
			return ""
		})
	}

	return strings.TrimSpace(sb.String())
}
