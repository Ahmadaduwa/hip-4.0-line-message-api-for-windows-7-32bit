package db

import (
	"time"

	"stw_teacher_check/attendance"
)

type MockDB struct {
	companyName string
}

func NewMockDB(companyName string) *MockDB {
	if companyName == "" {
		companyName = "STW_TeacherCheck (จำลอง)"
	}
	return &MockDB{companyName: companyName}
}

func (m *MockDB) Close() error {
	return nil
}

func (m *MockDB) TestConnection() map[string]interface{} {
	return map[string]interface{}{
		"ok":             true,
		"message":        "เชื่อมต่อโหมดจำลอง (Mock DB) สำเร็จ ข้อมูลเสมือนจริงพร้อมใช้งาน",
		"server_version": "MOCK-SERVER\\SQLEXPRESS (จำลอง)",
		"database":       "hiptime",
		"school_name":    m.companyName,
		"employee_count": 7,
	}
}

func (m *MockDB) GetSchool() (string, [7]int, error) {
	// Mon - Fri workdays (1 = workday, 0 = off)
	return m.companyName, [7]int{0, 1, 1, 1, 1, 1, 0}, nil
}

func (m *MockDB) GetPeople() ([]attendance.Person, error) {
	// พนักงานจำลองอิงตามหน้าจอ HIP Time 4.0 จริง
	return []attendance.Person{
		{Enroll: 1, Code: "1", Name: "นายสมชาย (1)", Level: "ADMIN", ShiftCode: "01"},
		{Enroll: 2, Code: "2", Name: "นางสาวสมศรี (2)", Level: "OFFICE", ShiftCode: "01"},
		{Enroll: 3, Code: "3", Name: "นายมานะ (3)", Level: "OFFICE", ShiftCode: "01"},
		{Enroll: 10, Code: "10", Name: "นายวิชัย (10)", Level: "SECURITY", ShiftCode: "01"},
		{Enroll: 99, Code: "99", Name: "นางสุภา (99)", Level: "OFFICE", ShiftCode: "01"},
		{Enroll: 159, Code: "jw", Name: "นายจิรวัฒน์ (jw)", Level: "TECH", ShiftCode: "01"},
		{Enroll: 1111, Code: "ade", Name: "นายอดิศักดิ์ (ade)", Level: "TECH", ShiftCode: "01"},
	}, nil
}

func (m *MockDB) GetShifts() (map[string]attendance.Shift, error) {
	return map[string]attendance.Shift{
		"01": {Code: "01", Name: "กะปกติ"},
		"99": {Code: "99", Name: "กะวันหยุด", IsDayOff: true},
	}, nil
}

func (m *MockDB) GetDailyShift(day time.Time) (map[int]string, error) {
	return map[int]string{}, nil
}

func (m *MockDB) GetLeaves(day time.Time) (map[string]string, error) {
	if day.Weekday() != time.Saturday && day.Weekday() != time.Sunday {
		return map[string]string{
			"99": "ลาป่วย",
		}, nil
	}
	return map[string]string{}, nil
}

func (m *MockDB) GetHolidays() (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *MockDB) GetScans(day time.Time) (map[int][]time.Time, error) {
	if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		return map[int][]time.Time{}, nil
	}

	loc := day.Location()
	y, mth, d := day.Year(), day.Month(), day.Day()

	// คนที่ 1: สแกน 07:15 -> ทันเวลา (ก่อน 07:30)
	// คนที่ 2: สแกน 07:42 -> สาย (หลัง 07:30)
	// คนที่ 3: สแกน 07:25 -> ทันเวลา (ก่อน 07:30)
	// คนที่ 159: สแกน 07:10 และ 16:35 -> ทันเวลา
	// คนที่ 1111: สแกน 07:55 -> สาย
	// คนที่ 10: ไม่สแกน -> ขาด
	// คนที่ 99: ลาป่วย
	return map[int][]time.Time{
		1: {
			time.Date(y, mth, d, 7, 15, 0, 0, loc),
			time.Date(y, mth, d, 16, 30, 0, 0, loc),
		},
		2: {
			time.Date(y, mth, d, 7, 42, 0, 0, loc),
		},
		3: {
			time.Date(y, mth, d, 7, 25, 0, 0, loc),
		},
		159: {
			time.Date(y, mth, d, 7, 10, 0, 0, loc),
			time.Date(y, mth, d, 16, 35, 0, 0, loc),
		},
		1111: {
			time.Date(y, mth, d, 7, 55, 0, 0, loc),
		},
	}, nil
}

func (m *MockDB) GetAutoLoadTimes() ([]string, error) {
	return []string{"09:00", "18:00"}, nil
}
