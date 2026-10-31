package report

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"stw_teacher_check/attendance"
	"stw_teacher_check/logger"

	"github.com/xuri/excelize/v2"
)

func TestExportDailyExcelAndRetention(t *testing.T) {
	testDir := "test_reports"
	_ = os.RemoveAll(testDir)
	defer os.RemoveAll(testDir)

	now := time.Date(2026, 10, 27, 9, 5, 0, 0, logger.BangkokLoc)
	day := now

	// สร้างไฟล์เก่าอายุ 35 วัน (ควรถูกลบ)
	_ = os.MkdirAll(testDir, 0755)
	oldFile := filepath.Join(testDir, fmt.Sprintf("attendance-%s.xlsx", now.AddDate(0, 0, -35).Format("2006-01-02")))
	_ = os.WriteFile(oldFile, []byte("old content"), 0644)

	// สร้างไฟล์อายุ 10 วัน (ต้องยังอยู่)
	recentFile := filepath.Join(testDir, fmt.Sprintf("attendance-%s.xlsx", now.AddDate(0, 0, -10).Format("2006-01-02")))
	_ = os.WriteFile(recentFile, []byte("recent content"), 0644)

	// ข้อมูลจำลอง
	tScanNormal := time.Date(2026, 10, 27, 7, 15, 0, 0, logger.BangkokLoc)
	tScanLate := time.Date(2026, 10, 27, 7, 45, 0, 0, logger.BangkokLoc)

	summary := attendance.Summary{
		Day:       day,
		IsWorkday: true,
		Results: []attendance.Result{
			{
				Person:    attendance.Person{Enroll: 1, Code: "001", Name: "นาย ก (ปกติ)"},
				Status:    "present",
				FirstScan: &tScanNormal,
			},
			{
				Person:      attendance.Person{Enroll: 2, Code: "002", Name: "นาย ข (สาย)"},
				Status:      "late",
				LateMinutes: 15,
				FirstScan:   &tScanLate,
			},
			{
				Person: attendance.Person{Enroll: 3, Code: "003", Name: "นาย ค (ขาด)"},
				Status: "absent",
			},
			{
				Person:    attendance.Person{Enroll: 4, Code: "004", Name: "นาย ง (ลา)"},
				Status:    "leave",
				LeaveName: "ลากิจ",
			},
		},
		TotalScans: 2,
	}

	excelPath, err := ExportDailyExcel(summary, "07:30", "โรงเรียนแสงธรรมวิทยา", testDir, 30, now)
	if err != nil {
		t.Fatalf("ExportDailyExcel error: %v", err)
	}

	// 1. ตรวจสอบว่าไฟล์เก่า 35 วันถูกลบ
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old file %s to be deleted", oldFile)
	}

	// 2. ตรวจสอบว่าไฟล์ 10 วันยังคงอยู่
	if _, err := os.Stat(recentFile); err != nil {
		t.Errorf("expected recent file %s to exist", recentFile)
	}

	// 3. ตรวจสอบเนื้อหาในไฟล์ Excel ที่สร้าง
	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		t.Fatalf("failed to open generated excel file: %v", err)
	}
	defer f.Close()

	sheetName := "สรุปการเข้างาน"

	// ตรวจสอบหัวเรื่อง
	valA1, _ := f.GetCellValue(sheetName, "A1")
	if valA1 != "รายงานการเข้างาน: โรงเรียนแสงธรรมวิทยา" {
		t.Errorf("unexpected A1 value: %s", valA1)
	}

	// ตรวจสอบเกณฑ์การตั้งค่า
	valA3, _ := f.GetCellValue(sheetName, "A3")
	if valA3 != "📌 เกณฑ์เวลาเข้างาน: มาปกติคือก่อน 07:30 น. | มาสายคือหลัง 07:30 น. (ตามที่ตั้งค่าไว้ในระบบ)" {
		t.Errorf("unexpected A3 value: %s", valA3)
	}

	// ตรวจสอบหัวตาราง 3 คอลัมน์ (ชื่อ, สถานะ, เวลาที่เข้า)
	h1, _ := f.GetCellValue(sheetName, "A5")
	h2, _ := f.GetCellValue(sheetName, "B5")
	h3, _ := f.GetCellValue(sheetName, "C5")
	if h1 != "ชื่อ" || h2 != "สถานะ" || h3 != "เวลาที่เข้า" {
		t.Errorf("unexpected headers: %s, %s, %s", h1, h2, h3)
	}

	// ตรวจสอบลำดับแถว (สาย -> ขาด -> ลา -> ปกติ)
	// แถวที่ 6: สาย (นาย ข)
	r6Name, _ := f.GetCellValue(sheetName, "A6")
	r6Status, _ := f.GetCellValue(sheetName, "B6")
	r6Time, _ := f.GetCellValue(sheetName, "C6")
	if r6Name != "นาย ข (สาย)" || r6Status != "สาย (15 นาที)" || r6Time != "07:45:00 น." {
		t.Errorf("row 6 mismatch: %s | %s | %s", r6Name, r6Status, r6Time)
	}

	// แถวที่ 7: ขาด (นาย ค)
	r7Name, _ := f.GetCellValue(sheetName, "A7")
	r7Status, _ := f.GetCellValue(sheetName, "B7")
	r7Time, _ := f.GetCellValue(sheetName, "C7")
	if r7Name != "นาย ค (ขาด)" || r7Status != "ขาด" || r7Time != "-" {
		t.Errorf("row 7 mismatch: %s | %s | %s", r7Name, r7Status, r7Time)
	}

	// แถวที่ 8: ลา (นาย ง)
	r8Name, _ := f.GetCellValue(sheetName, "A8")
	r8Status, _ := f.GetCellValue(sheetName, "B8")
	r8Time, _ := f.GetCellValue(sheetName, "C8")
	if r8Name != "นาย ง (ลา)" || r8Status != "ลา (ลากิจ)" || r8Time != "-" {
		t.Errorf("row 8 mismatch: %s | %s | %s", r8Name, r8Status, r8Time)
	}

	// แถวที่ 9: ปกติ (นาย ก)
	r9Name, _ := f.GetCellValue(sheetName, "A9")
	r9Status, _ := f.GetCellValue(sheetName, "B9")
	r9Time, _ := f.GetCellValue(sheetName, "C9")
	if r9Name != "นาย ก (ปกติ)" || r9Status != "ปกติ" || r9Time != "07:15:00 น." {
		t.Errorf("row 9 mismatch: %s | %s | %s", r9Name, r9Status, r9Time)
	}
}
