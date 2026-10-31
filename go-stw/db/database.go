package db

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	mssql "github.com/denisenkom/go-mssqldb"
	"github.com/denisenkom/go-mssqldb/msdsn"

	"stw_teacher_check/attendance"
	"stw_teacher_check/config"
	"stw_teacher_check/logger"
)

type Database interface {
	TestConnection() map[string]interface{}
	GetSchool() (string, [7]int, error)
	GetPeople() ([]attendance.Person, error)
	GetShifts() (map[string]attendance.Shift, error)
	GetDailyShift(day time.Time) (map[int]string, error)
	GetLeaves(day time.Time) (map[string]string, error)
	GetHolidays() (map[string]string, error)
	GetScans(day time.Time) (map[int][]time.Time, error)
	GetAutoLoadTimes() ([]string, error)
	Close() error
}

type SQLServerDB struct {
	db       *sql.DB
	host     string
	database string
}

func openSQLDB(connStr string) (*sql.DB, error) {
	msdsnCfg, _, err := msdsn.Parse(connStr)
	if err != nil {
		return nil, err
	}

	// กำหนด TLSConfig ให้รองรับ TLS 1.0 (0x0301) สำหรับ SQL Server 2008/2012 บน Windows 7
	// แม้เมื่อเซิร์ฟเวอร์บังคับเข้ารหัส Login Packet
	msdsnCfg.TLSConfig = &tls.Config{
		InsecureSkipVerify:          true,
		MinVersion:                  tls.VersionTLS10,
		MaxVersion:                  tls.VersionTLS13,
		DynamicRecordSizingDisabled: true,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}

	connector := mssql.NewConnectorConfig(msdsnCfg)
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(3)
	db.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func uniqueStrings(items ...string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" || seen[strings.ToLower(it)] {
			continue
		}
		seen[strings.ToLower(it)] = true
		out = append(out, it)
	}
	return out
}

func Connect(cfg config.Settings) (Database, error) {
	if cfg.MockMode || strings.EqualFold(cfg.DBAuth, "mock") || strings.EqualFold(cfg.DBHost, "mock") {
		logger.Info("DB", "ใช้ฐานข้อมูลจำลอง (Mock DB)")
		return NewMockDB(cfg.CompanyName), nil
	}

	// 1. ลองเชื่อมต่อด้วยค่าที่ตั้งไว้ก่อน
	connStr := buildConnString(cfg)
	primaryDB, err := openSQLDB(connStr)
	if err == nil {
		if pingErr := primaryDB.Ping(); pingErr == nil {
			logger.Info("DB", "✅ เชื่อมต่อ SQL Server สำเร็จ (%s / DB: %s)", cfg.DBHost, cfg.DBName)
			return &SQLServerDB{
				db:       primaryDB,
				host:     cfg.DBHost,
				database: cfg.DBName,
			}, nil
		} else {
			_ = primaryDB.Close()
			err = pingErr
		}
	}

	// 2. Auto-Fallback: ลองสลับชื่อ DB (Synctime / hiptime) และโหมดการยืนยันตัวตนอัตโนมัติ
	hosts := uniqueStrings(cfg.DBHost, "127.0.0.1", `.\SQLEXPRESS`, `SS-PC\SQLEXPRES`)
	dbNames := uniqueStrings(cfg.DBName, "Synctime", "hiptime")
	passwords := uniqueStrings(cfg.DBPassword)

	for _, h := range hosts {
		for _, dbName := range dbNames {
			// ลอง SQL Auth
			for _, pw := range passwords {
				tryCfg := cfg
				tryCfg.DBHost = h
				tryCfg.DBName = dbName
				tryCfg.DBAuth = "sql"
				tryCfg.DBUser = "sa"
				tryCfg.DBPassword = pw

				cStr := buildConnString(tryCfg)
				d, oErr := openSQLDB(cStr)
				if oErr != nil {
					continue
				}
				if pErr := d.Ping(); pErr == nil {
					logger.Info("DB", "✅ เชื่อมต่อ SQL Server สำเร็จผ่านระบบ Auto-Fallback (Host: %s, DB: %s)", h, dbName)
					return &SQLServerDB{
						db:       d,
						host:     h,
						database: dbName,
					}, nil
				}
				_ = d.Close()
			}

			// ลอง Windows Auth
			winCfg := cfg
			winCfg.DBHost = h
			winCfg.DBName = dbName
			winCfg.DBAuth = "windows"
			winCfg.DBPassword = ""
			cStr := buildConnString(winCfg)
			if d, oErr := openSQLDB(cStr); oErr == nil {
				if pErr := d.Ping(); pErr == nil {
					logger.Info("DB", "✅ เชื่อมต่อ SQL Server สำเร็จผ่าน Windows Auth (Host: %s, DB: %s)", h, dbName)
					return &SQLServerDB{
						db:       d,
						host:     h,
						database: dbName,
					}, nil
				}
				_ = d.Close()
			}
		}
	}

	// หากไม่สำเร็จทั้งหมด ให้คืน instance ของค่าหลักเพื่อให้ TestConnection() รายงานข้อผิดพลาดที่แท้จริงได้
	fallbackDB, _ := openSQLDB(connStr)
	logger.Error("DB", err, "เปิดการเชื่อมต่อ SQL Server ไม่สำเร็จ: %s (DB: %s)", cfg.DBHost, cfg.DBName)
	return &SQLServerDB{
		db:       fallbackDB,
		host:     cfg.DBHost,
		database: cfg.DBName,
	}, err
}

func buildConnString(cfg config.Settings) string {
	server := strings.TrimSpace(cfg.DBHost)
	if server == "" {
		server = "127.0.0.1"
	}
	// ปรับ backslash ให้ถูกต้อง ไม่ให้ซ้ำซ้อน
	server = regexp.MustCompile(`\\+`).ReplaceAllString(server, `\`)

	dbName := strings.TrimSpace(cfg.DBName)
	if dbName == "" {
		dbName = "Synctime"
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("server=%s", server))

	// ถ้าไม่มี instance และระบุ port
	if cfg.DBPort > 0 && !strings.Contains(server, `\`) {
		parts = append(parts, fmt.Sprintf("port=%d", cfg.DBPort))
	}
	parts = append(parts, fmt.Sprintf("database=%s", dbName))
	parts = append(parts, "encrypt=disable")
	parts = append(parts, "trustservercertificate=true")
	parts = append(parts, "connection timeout=5")
	parts = append(parts, "dial timeout=5")

	// Authentication mode
	if strings.EqualFold(cfg.DBAuth, "windows") || cfg.DBPassword == "" {
		parts = append(parts, "trusted_connection=yes")
	} else {
		user := strings.TrimSpace(cfg.DBUser)
		if user == "" {
			user = "sa"
		}
		parts = append(parts, fmt.Sprintf("user id=%s", user))
		parts = append(parts, fmt.Sprintf("password=%s", cfg.DBPassword))
	}

	return strings.Join(parts, ";") + ";"
}

func (s *SQLServerDB) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *SQLServerDB) TestConnection() map[string]interface{} {
	if s.db == nil {
		return map[string]interface{}{
			"ok":       false,
			"error":    "ยังไม่ได้กำหนดการเชื่อมต่อฐานข้อมูล",
			"advice":   "ตรวจสอบค่าตั้งค่าฐานข้อมูล",
			"host":     s.host,
			"database": s.database,
		}
	}
	if err := s.db.Ping(); err != nil {
		errStr := err.Error()
		advice := "ตรวจสอบชื่อ Server/Instance และเปิดพอร์ต 1433 ใน SQL Server Configuration Manager"
		if strings.Contains(errStr, "Sql Server Browser") {
			advice = "บริการ 'SQL Server Browser' ปิดอยู่ กรุณาคลิกขวาที่ไฟล์ fix_sql_connection.bat แล้วเลือก 'Run as administrator' หรือเปิด services.msc สั่ง Start บริการ SQL Server Browser"
		} else if strings.Contains(errStr, "Login failed") {
			advice = "รหัสผ่าน SQL Server ('sa') ไม่ถูกต้อง หรือ Windows User ไม่มีสิทธิ์ sysadmin"
		} else if strings.Contains(errStr, "Cannot open database") {
			advice = fmt.Sprintf("ไม่พบฐานข้อมูล '%s' (ตรวจสอบว่าชื่อ Synctime หรือ hiptime ตรงกับในเครื่องหรือไม่)", s.database)
		}

		logger.Error("DB", err, "ทดสอบเชื่อมต่อล้มเหลว: %s", advice)
		return map[string]interface{}{
			"ok":       false,
			"error":    errStr,
			"advice":   advice,
			"host":     s.host,
			"database": s.database,
		}
	}

	var ver string
	var dbName string
	_ = s.db.QueryRow("SELECT @@VERSION, DB_NAME()").Scan(&ver, &dbName)
	if lines := strings.Split(ver, "\n"); len(lines) > 0 {
		ver = strings.TrimSpace(lines[0])
	}
	if dbName == "" {
		dbName = s.database
	}

	people, _ := s.GetPeople()
	schoolName, _, _ := s.GetSchool()

	return map[string]interface{}{
		"ok":             true,
		"message":        "เชื่อมต่อฐานข้อมูล SQL Server สำเร็จเรียบร้อย",
		"server_version": ver,
		"host":           s.host,
		"database":       dbName,
		"school_name":    schoolName,
		"employee_count": len(people),
	}
}

func (s *SQLServerDB) GetSchool() (string, [7]int, error) {
	var name sql.NullString
	var mon, tue, wed, thu, fri, sat, sun sql.NullInt64

	row := s.db.QueryRow(`
		SELECT TOP 1 schoolname, monwork, tuework, wedwork, thuwork, friwork, satwork, sunwork 
		FROM School 
		WHERE active = 1 OR active IS NULL 
		ORDER BY id
	`)
	err := row.Scan(&name, &mon, &tue, &wed, &thu, &fri, &sat, &sun)
	if err != nil && err != sql.ErrNoRows {
		return "", [7]int{0, 1, 1, 1, 1, 1, 0}, err
	}

	// Index 0: Sunday, 1: Monday, ... 6: Saturday
	workdays := [7]int{
		int(sun.Int64), int(mon.Int64), int(tue.Int64),
		int(wed.Int64), int(thu.Int64), int(fri.Int64), int(sat.Int64),
	}
	if !mon.Valid {
		workdays = [7]int{0, 1, 1, 1, 1, 1, 0}
	}

	schoolName := strings.TrimSpace(name.String)
	if schoolName == "" || strings.Contains(strings.ToUpper(schoolName), "HIP") {
		schoolName = "โรงเรียนแสงธรรมวิทยา"
	}

	return schoolName, workdays, nil
}

func (s *SQLServerDB) GetPeople() ([]attendance.Person, error) {
	rows, err := s.db.Query(`
		SELECT enrollnumber, studentcode, studentname, studentnamee, levelcode, shiftcode 
		FROM Student 
		WHERE (active = 1 OR active IS NULL) AND enrollnumber IS NOT NULL AND enrollnumber > 0
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var people []attendance.Person
	for rows.Next() {
		var enroll int
		var code, name, nameEn, level, shift sql.NullString
		if err := rows.Scan(&enroll, &code, &name, &nameEn, &level, &shift); err != nil {
			continue
		}
		pName := strings.TrimSpace(name.String)
		if pName == "" {
			pName = strings.TrimSpace(nameEn.String)
		}
		if pName == "" {
			pName = fmt.Sprintf("รหัส %d", enroll)
		}
		people = append(people, attendance.Person{
			Enroll:    enroll,
			Code:      strings.TrimSpace(code.String),
			Name:      pName,
			Level:     strings.TrimSpace(level.String),
			ShiftCode: strings.TrimSpace(shift.String),
		})
	}
	return people, nil
}

func (s *SQLServerDB) GetShifts() (map[string]attendance.Shift, error) {
	rows, err := s.db.Query(`SELECT shiftcode, shiftname, starttime, endtime FROM Shift`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	shifts := make(map[string]attendance.Shift)
	for rows.Next() {
		var code, name, start, end sql.NullString
		if err := rows.Scan(&code, &name, &start, &end); err != nil {
			continue
		}
		c := strings.TrimSpace(code.String)
		sh := attendance.Shift{
			Code: c,
			Name: strings.TrimSpace(name.String),
		}
		if sh.Name == "" {
			sh.Name = c
		}
		if start.Valid && end.Valid {
			if start.String == "00:00" && end.String == "00:00" {
				sh.IsDayOff = true
			}
		}
		shifts[c] = sh
	}
	return shifts, nil
}

func (s *SQLServerDB) GetDailyShift(day time.Time) (map[int]string, error) {
	dateStr := day.Format("2006-01-02")
	rows, err := s.db.Query(`SELECT enrollnumber, tdate, shiftcode FROM StudentShift`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int]string)
	for rows.Next() {
		var enroll int
		var tdate, shift sql.NullString
		if err := rows.Scan(&enroll, &tdate, &shift); err != nil {
			continue
		}
		if enroll > 0 && strings.HasPrefix(strings.TrimSpace(tdate.String), dateStr) {
			m[enroll] = strings.TrimSpace(shift.String)
		}
	}
	return m, nil
}

func (s *SQLServerDB) GetLeaves(day time.Time) (map[string]string, error) {
	dateStr := day.Format("2006-01-02")
	rows, err := s.db.Query(`
		SELECT tl.studentcode, l.leavename, tl.leavedate 
		FROM TranLeave tl 
		LEFT JOIN Leave l ON tl.leavecode = l.leavecode
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var code, name, ldate sql.NullString
		if err := rows.Scan(&code, &name, &ldate); err != nil {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(ldate.String), dateStr) {
			lName := strings.TrimSpace(name.String)
			if lName == "" {
				lName = "ลา"
			}
			m[strings.TrimSpace(code.String)] = lName
		}
	}
	return m, nil
}

func (s *SQLServerDB) GetHolidays() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT holdate, holreason FROM Holiday`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var hdate, reason sql.NullString
		if err := rows.Scan(&hdate, &reason); err != nil {
			continue
		}
		dStr := parseDateStr(hdate.String)
		if dStr != "" {
			r := strings.TrimSpace(reason.String)
			if r == "" {
				r = "วันหยุด"
			}
			m[dStr] = r
		}
	}
	return m, nil
}

func (s *SQLServerDB) GetAutoLoadTimes() ([]string, error) {
	rows, err := s.db.Query(`SELECT loadtime FROM AutoLoadTime WHERE loadtime IS NOT NULL AND loadtime != '' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var times []string
	for rows.Next() {
		var lt sql.NullString
		if err := rows.Scan(&lt); err != nil {
			continue
		}
		tStr := strings.TrimSpace(lt.String)
		if tStr != "" {
			times = append(times, tStr)
		}
	}
	return times, nil
}

var BangkokLoc = time.FixedZone("Asia/Bangkok", 7*3600)

func (s *SQLServerDB) GetScans(day time.Time) (map[int][]time.Time, error) {
	day = day.In(BangkokLoc)
	dayStr := day.Format("2006-01-02")
	thaiYear := day.Year() + 543
	dayThaiStr := fmt.Sprintf("%04d-%s", thaiYear, day.Format("01-02"))
	cutoff := day.AddDate(0, 0, -1).Format("2006-01-02")

	rows, err := s.db.Query(`
		SELECT enrollnumber, datetimescan 
		FROM Transcantime 
		WHERE datetimescan LIKE @p1 
		   OR datetimescan LIKE @p2 
		   OR adddate >= @p3 
		   OR adddate IS NULL
	`, dayStr+"%", dayThaiStr+"%", cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int][]time.Time)

	for rows.Next() {
		var enroll int
		var scanStr sql.NullString
		if err := rows.Scan(&enroll, &scanStr); err != nil {
			continue
		}
		if enroll <= 0 || !scanStr.Valid {
			continue
		}
		t := parseDateTime(scanStr.String)
		if !t.IsZero() && t.In(BangkokLoc).Format("2006-01-02") == dayStr {
			m[enroll] = append(m[enroll], t)
		}
	}
	return m, nil
}

// ================== DATE/TIME PARSING ==================

var dtFormats = []string{
	"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04:05",
	"2006/01/02 15:04:05", "2006/01/02 15:04",
	"02/01/2006 15:04:05", "02/01/2006 15:04",
	"02-01-2006 15:04:05", "02-01-2006 15:04",
	"2006-01-02", "2006/01/02", "02/01/2006", "02-01-2006",
}

var (
	reMS   = regexp.MustCompile(`\.\d+`)
	reAMPM = regexp.MustCompile(`(?i)\s*(AM|PM)\s*$`)
	reYear = regexp.MustCompile(`^(\d{4})([-/])`)
)

func parseDateTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	// ตัด ms เช่น .177
	s = reMS.ReplaceAllString(s, "")

	// ตรวจสอบ พ.ศ. (ปี > 2400 เช่น 2569) แปลงเป็น ค.ศ. ก่อน parse
	if m := reYear.FindStringSubmatch(s); len(m) == 3 {
		if y, err := strconv.Atoi(m[1]); err == nil && y > 2400 {
			s = fmt.Sprintf("%04d%s%s", y-543, m[2], s[len(m[0]):])
		}
	}

	// ลบ AM / PM ออก (เนื่องจากเวลาจาก HIP/Windows มักบันทึกเป็น 24 ชม. อยู่แล้ว เช่น 17:53:14 PM)
	s = strings.TrimSpace(reAMPM.ReplaceAllString(s, ""))

	for _, fmtStr := range dtFormats {
		t, err := time.ParseInLocation(fmtStr, s, BangkokLoc)
		if err == nil {
			if t.Year() > 2400 { // พ.ศ.
				t = t.AddDate(-543, 0, 0)
			}
			return t
		}
	}
	return time.Time{}
}

func parseDateStr(s string) string {
	t := parseDateTime(s)
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
