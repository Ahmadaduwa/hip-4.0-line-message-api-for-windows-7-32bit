package report

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"stw_teacher_check/attendance"
	"stw_teacher_check/logger"

	"github.com/xuri/excelize/v2"
)

// ExportDailyExcel สร้างไฟล์ Excel ประจำวัน 3 คอลัมน์ (ชื่อ, สถานะ, เวลาที่เข้า)
// พร้อมข้อความระบุเกณฑ์การตั้งค่าด้านบน และลบไฟล์ย้อนหลังที่เกิน retentionDays วัน
func ExportDailyExcel(
	summary attendance.Summary,
	lateTime string,
	companyName string,
	reportsDir string,
	retentionDays int,
	now time.Time,
) (string, error) {
	if reportsDir == "" {
		reportsDir = "reports"
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}
	if lateTime == "" {
		lateTime = "07:30"
	}
	if companyName == "" {
		companyName = "โรงเรียนแสงธรรมวิทยา"
	}

	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return "", fmt.Errorf("สร้างโฟลเดอร์เก็บรายงานไม่สำเร็จ: %w", err)
	}

	// หมุนเวียนลบไฟล์ที่เกิน retentionDays ย้อนหลัง
	CleanOldReports(reportsDir, retentionDays, now)

	dateStr := summary.Day.Format("2006-01-02")
	filePath := filepath.Join(reportsDir, fmt.Sprintf("attendance-%s.xlsx", dateStr))

	f := excelize.NewFile()
	defer func() {
		_ = f.Close()
	}()

	sheetName := "สรุปการเข้างาน"
	f.SetSheetName("Sheet1", sheetName)

	// Styles
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Color: "1F497D"},
	})
	subTitleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Size: 10, Color: "595959"},
	})
	ruleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10, Color: "C00000"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"F2F2F2"}, Pattern: 1},
	})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"366092"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "D9D9D9", Style: 1},
			{Type: "top", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
			{Type: "right", Color: "D9D9D9", Style: 1},
		},
	})
	cellLeft, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "E0E0E0", Style: 1},
			{Type: "top", Color: "E0E0E0", Style: 1},
			{Type: "bottom", Color: "E0E0E0", Style: 1},
			{Type: "right", Color: "E0E0E0", Style: 1},
		},
	})
	cellCenter, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "E0E0E0", Style: 1},
			{Type: "top", Color: "E0E0E0", Style: 1},
			{Type: "bottom", Color: "E0E0E0", Style: 1},
			{Type: "right", Color: "E0E0E0", Style: 1},
		},
	})

	// 1. หัวข้อด้านบน
	_ = f.SetCellValue(sheetName, "A1", fmt.Sprintf("รายงานการเข้างาน: %s", companyName))
	_ = f.SetCellStyle(sheetName, "A1", "A1", titleStyle)

	_ = f.SetCellValue(sheetName, "A2", fmt.Sprintf("ประจำวันที่: %s (สร้างเมื่อ %s น.)", attendance.ThaiDate(summary.Day), now.Format("15:04:05")))
	_ = f.SetCellStyle(sheetName, "A2", "A2", subTitleStyle)

	// 2. แสดงเกณฑ์การตั้งค่าตามหน้าเว็บ
	ruleText := fmt.Sprintf("📌 เกณฑ์เวลาเข้างาน: มาปกติคือก่อน %s น. | มาสายคือหลัง %s น. (ตามที่ตั้งค่าไว้ในระบบ)", lateTime, lateTime)
	_ = f.SetCellValue(sheetName, "A3", ruleText)
	_ = f.SetCellStyle(sheetName, "A3", "C3", ruleStyle)

	// 3. หัวตาราง (3 คอลัมน์: ชื่อ, สถานะ, เวลาที่เข้า)
	headerRow := 5
	headers := []string{"ชื่อ", "สถานะ", "เวลาที่เข้า"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
		_ = f.SetCellValue(sheetName, cell, h)
		_ = f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// 4. จัดเรียงข้อมูลตามกลุ่มสถานะ: สาย -> ขาด -> ลา -> ปกติ -> อื่นๆ
	results := make([]attendance.Result, len(summary.Results))
	copy(results, summary.Results)

	sortPriority := func(status string) int {
		switch status {
		case "late":
			return 1
		case "absent":
			return 2
		case "leave":
			return 3
		case "present":
			return 4
		case "pending":
			return 5
		case "dayoff":
			return 6
		default:
			return 7
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		pi := sortPriority(results[i].Status)
		pj := sortPriority(results[j].Status)
		if pi != pj {
			return pi < pj
		}
		// ในกลุ่มสาย เรียงคนสายมาก่อน
		if results[i].Status == "late" && results[j].Status == "late" {
			if results[i].LateMinutes != results[j].LateMinutes {
				return results[i].LateMinutes > results[j].LateMinutes
			}
		}
		// ในกลุ่มปกติ เรียงตามเวลาเข้า
		if results[i].Status == "present" && results[j].Status == "present" {
			if results[i].FirstScan != nil && results[j].FirstScan != nil {
				return results[i].FirstScan.Before(*results[j].FirstScan)
			}
		}
		return results[i].Person.Name < results[j].Person.Name
	})

	// 5. เขียนข้อมูลลงในตาราง
	rowIdx := headerRow + 1
	for _, r := range results {
		name := r.Person.Name
		if name == "" {
			name = fmt.Sprintf("รหัส %s", r.Person.Code)
		}

		statusText := formatStatusThai(r)
		timeText := "-"
		if r.FirstScan != nil {
			timeText = r.FirstScan.Format("15:04:05 น.")
		}

		cName, _ := excelize.CoordinatesToCellName(1, rowIdx)
		cStatus, _ := excelize.CoordinatesToCellName(2, rowIdx)
		cTime, _ := excelize.CoordinatesToCellName(3, rowIdx)

		_ = f.SetCellValue(sheetName, cName, name)
		_ = f.SetCellValue(sheetName, cStatus, statusText)
		_ = f.SetCellValue(sheetName, cTime, timeText)

		_ = f.SetCellStyle(sheetName, cName, cName, cellLeft)
		_ = f.SetCellStyle(sheetName, cStatus, cStatus, cellCenter)
		_ = f.SetCellStyle(sheetName, cTime, cTime, cellCenter)

		rowIdx++
	}

	// 6. กำหนดความกว้างคอลัมน์ให้อ่านง่าย
	_ = f.SetColWidth(sheetName, "A", "A", 32)
	_ = f.SetColWidth(sheetName, "B", "B", 20)
	_ = f.SetColWidth(sheetName, "C", "C", 20)

	// 7. บันทึกไฟล์
	if err := f.SaveAs(filePath); err != nil {
		return "", fmt.Errorf("บันทึกไฟล์ Excel ไม่สำเร็จ: %w", err)
	}

	return filePath, nil
}

// formatStatusThai แปลงสถานะเป็นข้อความภาษาไทยที่ชัดเจน
func formatStatusThai(r attendance.Result) string {
	switch r.Status {
	case "present":
		return "ปกติ"
	case "late":
		if r.LateMinutes > 0 {
			return fmt.Sprintf("สาย (%d นาที)", r.LateMinutes)
		}
		return "สาย"
	case "absent":
		return "ขาด"
	case "leave":
		if r.LeaveName != "" {
			return fmt.Sprintf("ลา (%s)", r.LeaveName)
		}
		return "ลา"
	case "pending":
		return "ยังไม่ถึงเวลา"
	case "dayoff":
		return "วันหยุด"
	default:
		return r.Status
	}
}

// CleanOldReports ลบไฟล์ Excel ใน reportsDir ที่เก่าเกิน retentionDays วัน
func CleanOldReports(reportsDir string, retentionDays int, now time.Time) {
	cutoff := now.AddDate(0, 0, -retentionDays)
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "attendance-") && strings.HasSuffix(name, ".xlsx") {
			datePart := strings.TrimSuffix(strings.TrimPrefix(name, "attendance-"), ".xlsx")
			t, err := time.ParseInLocation("2006-01-02", datePart, logger.BangkokLoc)
			if err == nil && t.Before(cutoff) {
				_ = os.Remove(filepath.Join(reportsDir, name))
			}
		}
	}
}
