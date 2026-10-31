package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"stw_teacher_check/attendance"
	"stw_teacher_check/config"
	"stw_teacher_check/db"
	"stw_teacher_check/line"
	"stw_teacher_check/logger"
	"stw_teacher_check/report"
	"stw_teacher_check/storage"
	"stw_teacher_check/telegram"
	"stw_teacher_check/web"
)

var (
	currentDB   db.Database
	dbMu        sync.RWMutex
	webInstance *web.WebServer
	tgPoller    *telegram.Poller
	tgWatcher   *telegram.RealtimeWatcher
)

func getDB() db.Database {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return currentDB
}

func initDB(mock bool) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	cfg := config.LoadSettings()
	if mock {
		cfg.MockMode = true
	}

	if currentDB != nil {
		_ = currentDB.Close()
	}

	d, err := db.Connect(cfg)
	currentDB = d
	if err != nil {
		logger.Error("MAIN", err, "เชื่อมต่อฐานข้อมูล SQL Server ยังไม่สำเร็จ (กรุณาตรวจสอบการตั้งค่า DB)")
		return err
	}
	return nil
}

func makeReport(d db.Database, day time.Time, now time.Time) (attendance.Summary, map[int][]time.Time, string, error) {
	cfg := config.LoadSettings()

	people, err := d.GetPeople()
	if err != nil {
		return attendance.Summary{}, nil, "", err
	}
	scans, err := d.GetScans(day)
	if err != nil {
		return attendance.Summary{}, nil, "", err
	}
	shifts, _ := d.GetShifts()
	dailyShift, _ := d.GetDailyShift(day)
	leaves, _ := d.GetLeaves(day)
	holidays, _ := d.GetHolidays()
	schoolName, workdays, _ := d.GetSchool()
	if schoolName != "" && !strings.Contains(strings.ToUpper(schoolName), "HIP") {
		cfg.CompanyName = schoolName
	}
	if cfg.CompanyName == "" || strings.Contains(strings.ToUpper(cfg.CompanyName), "HIP") {
		cfg.CompanyName = "โรงเรียนแสงธรรมวิทยา"
	}

	summary := attendance.Compute(
		day, people, scans, shifts, dailyShift, leaves, holidays, workdays,
		cfg.LateTime, now,
	)

	msg := attendance.BuildMessage(summary, cfg.CompanyName, &now, cfg.ShowPresent)
	return summary, scans, msg, nil
}

func runOnce(day time.Time, dryRun bool, force bool) error {
	now := time.Now().In(db.BangkokLoc)
	day = day.In(db.BangkokLoc)
	d := getDB()
	if d == nil {
		logger.Warn("REPORT", "ฐานข้อมูลยังไม่ได้เชื่อมต่อ กำลังพยายามเชื่อมต่อใหม่อัตโนมัติ...")
		if err := initDB(false); err == nil {
			d = getDB()
		}
	}
	if d == nil {
		return fmt.Errorf("ฐานข้อมูลยังไม่พร้อมใช้งาน")
	}

	summary, scans, text, err := makeReport(d, day, now)
	if err != nil {
		// ลองเชื่อมต่อฐานข้อมูลใหม่อีก 1 ครั้ง เผื่อ connection หลุดระหว่าง idle
		logger.Warn("REPORT", "ดึงข้อมูลไม่สำเร็จ (%v) กำลังลองเชื่อมต่อฐานข้อมูลใหม่...", err)
		if reconnErr := initDB(false); reconnErr == nil {
			d = getDB()
			if d != nil {
				summary, scans, text, err = makeReport(d, day, now)
			}
		}
	}
	if err != nil {
		logger.Error("REPORT", err, "สร้างรายงานสรุปไม่สำเร็จ")
		return err
	}

	cfg := config.LoadSettings()

	// Dynamic Summary: สำหรับรอบส่งอัตโนมัติ (force == false) ให้ส่งเฉพาะเมื่อมีการเช็คชื่อเพิ่มขึ้นจากรอบก่อนหน้าเท่านั้น
	if !dryRun && !force {
		if !storage.HasNewSummaryScans(day, scans) {
			logger.Info("REPORT", "⏭️ [Dynamic Summary] รอบ %s: ไม่มีการเช็คชื่อเพิ่มจากรอบก่อนหน้า (รวมวันนี้ %d รายการ) ข้ามการส่งรายงานสรุป (LINE & Telegram)", now.Format("15:04"), summary.TotalScans)
			if webInstance != nil {
				webInstance.SetLastRun(now.Format("2006-01-02 15:04:05"), "ข้ามการส่ง (ไม่มีการเช็คชื่อเพิ่ม)")
				webInstance.SetTelegramLastRun(now.Format("2006-01-02 15:04:05"), "ข้ามการส่ง (ไม่มีการเช็คชื่อเพิ่ม)")
			}
			return nil
		}
	}

	// สร้างไฟล์ Excel สรุปประจำวันและจัดการ retention 30 วัน (เมื่อมีข้อมูลสแกนหรือสั่งส่งด้วยมือ)
	if summary.TotalScans > 0 || force {
		excelPath, err := report.ExportDailyExcel(summary, cfg.LateTime, cfg.CompanyName, "reports", 30, now)
		if err != nil {
			logger.Error("REPORT", err, "สร้างไฟล์ Excel ไม่สำเร็จ")
		} else {
			logger.Info("REPORT", "📊 บันทึกไฟล์ Excel ประจำวันสำเร็จ: %s", excelPath)
		}
	}

	if dryRun {
		fmt.Println("--- DRY RUN PREVIEW ---")
		fmt.Println(text)
		return nil
	}

	// 1. ส่งรายงานสรุปเข้า Telegram (หากเปิดใช้งาน) — แสดงรายชื่อครบ 100% ทุกคน (ไม่จำกัด 100 คน) และแยกสวิตช์ TelegramShowPresent อิสระจาก LINE
	tgToken := cfg.EffectiveTelegramToken()
	if cfg.TelegramEnabled && cfg.TelegramSummaryEnabled && tgToken != "" {
		tgText := attendance.BuildFullMessage(summary, cfg.CompanyName, &now, cfg.TelegramShowPresent)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		tgErr := telegram.DeliverSummary(ctx, tgToken, summary, tgText)
		cancel()
		if webInstance != nil {
			if tgErr != nil {
				webInstance.SetTelegramLastRun(now.Format("2006-01-02 15:04:05"), fmt.Sprintf("ส่งไม่สำเร็จ: %s", tgErr))
			} else {
				webInstance.SetTelegramLastRun(now.Format("2006-01-02 15:04:05"), "ส่งสำเร็จ")
			}
		}
	}

	// 2. ส่งรายงานสรุปเข้า LINE
	lineClient := line.NewClient(cfg.LineChannelToken)
	targets := storage.GetActiveTargetIDs()
	if len(targets) == 0 {
		logger.Info("REPORT", "ไม่มีรายชื่อผู้รับที่ระบุไว้ ทำการส่งแบบ Broadcast ถึงทุกคนที่เป็นเพื่อนกับ LINE Official Account")
		err := lineClient.BroadcastText(text)
		if err != nil {
			logger.Error("REPORT", err, "ส่ง LINE Broadcast ไม่สำเร็จ")
			if webInstance != nil {
				webInstance.SetLastRun(now.Format("2006-01-02 15:04:05"), fmt.Sprintf("ส่งไม่สำเร็จ: %s", err))
			}
			return err
		}
		storage.MarkSummaryScansSent(day, scans)
		logger.Info("REPORT", "ส่งรายงานประจำวันแบบ Broadcast สำเร็จเรียบร้อย")
		if webInstance != nil {
			webInstance.SetLastRun(now.Format("2006-01-02 15:04:05"), "ส่งสำเร็จ (Broadcast ทุกคน)")
		}
		return nil
	}

	err = lineClient.SendText(targets, text)
	if err != nil {
		logger.Error("REPORT", err, "ส่งรายงานเข้า LINE ไม่สำเร็จ")
		if webInstance != nil {
			webInstance.SetLastRun(now.Format("2006-01-02 15:04:05"), fmt.Sprintf("ส่งไม่สำเร็จ: %s", err))
		}
		return err
	}

	storage.MarkSummaryScansSent(day, scans)
	logger.Info("REPORT", "ส่งรายงานประจำวันไปยัง %d ผู้รับที่เปิดสวิตช์ สำเร็จเรียบร้อย", len(targets))
	if webInstance != nil {
		webInstance.SetLastRun(now.Format("2006-01-02 15:04:05"), fmt.Sprintf("ส่งสำเร็จ (%d ผู้รับ)", len(targets)))
	}
	return nil
}

func runTelegramOnce(day time.Time, force bool) error {
	now := time.Now().In(db.BangkokLoc)
	day = day.In(db.BangkokLoc)
	d := getDB()
	if d == nil {
		logger.Warn("TELEGRAM", "ฐานข้อมูลยังไม่ได้เชื่อมต่อ กำลังพยายามเชื่อมต่อใหม่อัตโนมัติ...")
		if err := initDB(false); err == nil {
			d = getDB()
		}
	}
	if d == nil {
		return fmt.Errorf("ฐานข้อมูลยังไม่ได้เชื่อมต่อ")
	}

	summary, scans, _, err := makeReport(d, day, now)
	if err != nil {
		return err
	}

	cfg := config.LoadSettings()
	tgText := attendance.BuildFullMessage(summary, cfg.CompanyName, &now, cfg.TelegramShowPresent)
	token := cfg.EffectiveTelegramToken()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err = telegram.DeliverSummary(ctx, token, summary, tgText)
	if err == nil {
		storage.MarkSummaryScansSent(day, scans)
	}
	if webInstance != nil {
		if err != nil {
			webInstance.SetTelegramLastRun(now.Format("2006-01-02 15:04:05"), fmt.Sprintf("ส่งไม่สำเร็จ: %s", err))
		} else {
			webInstance.SetTelegramLastRun(now.Format("2006-01-02 15:04:05"), "ส่งสำเร็จ")
		}
	}
	return err
}

func startScheduler() {
	lastSentMinute := ""
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().In(db.BangkokLoc)
		cfg := config.LoadSettings()

		currentMinute := now.Format("2006-01-02 15:04")
		if currentMinute == lastSentMinute {
			continue
		}

		currentH := now.Hour()
		currentM := now.Minute()

		shouldSend := false
		for _, tStr := range cfg.SendTimes {
			h, m, ok := attendance.ParseHM(tStr)
			if ok && h == currentH && m == currentM {
				shouldSend = true
				break
			}
		}

		if shouldSend {
			lastSentMinute = currentMinute
			logger.Info("SCHEDULER", "⏰ ถึงเวลาตรวจสอบรอบส่งรายงานสรุปประจำวัน (LINE & Telegram) (%s)", now.Format("15:04:05"))
			_ = runOnce(now, false, false)
		}
	}
}

func main() {
	// บังคับให้ Working Directory อยู่ข้างไฟล์ .exe เสมอ (สำคัญมากเมื่อรันผ่าน Task Scheduler บน Windows)
	if exePath, err := os.Executable(); err == nil {
		dir := filepath.Dir(exePath)
		if !strings.Contains(dir, "go-build") && !strings.Contains(dir, "/tmp") && !strings.Contains(dir, `\AppData\Local\Temp`) {
			_ = os.Chdir(dir)
		}
	}

	logger.InitLogger()
	logger.Info("MAIN", "==================================================")
	logger.Info("MAIN", "   เริ่มต้นระบบ STW_TeacherCheck (Go Edition)     ")
	logger.Info("MAIN", "==================================================")

	nowFlag := flag.Bool("now", false, "ส่งสรุปของวันนี้ทันที")
	mockFlag := flag.Bool("mock", false, "รันด้วยฐานข้อมูลจำลอง (เสมือนจริง)")
	dryRunFlag := flag.Bool("dry-run", false, "แสดงข้อความตัวอย่าง ไม่ส่ง LINE")
	dateFlag := flag.String("date", "", "ระบุวันที่สรุป (YYYY-MM-DD)")
	checkFlag := flag.Bool("check", false, "ทดสอบเชื่อมต่อฐานข้อมูล")
	tgCheckFlag := flag.Bool("telegram-check", false, "ทดสอบเชื่อมต่อ Telegram Bot API")
	flag.Parse()

	if *tgCheckFlag {
		cfg := config.LoadSettings()
		token := cfg.EffectiveTelegramToken()
		if token == "" {
			fmt.Println("❌ ไม่พบ TELEGRAM_TOKEN ใน .env หรือ settings.json")
			return
		}
		client := telegram.NewClient(token)
		user, err := client.GetMe(context.Background())
		if err != nil {
			fmt.Printf("❌ เชื่อมต่อ Telegram Bot ไม่สำเร็จ: %v\n", err)
			return
		}
		fmt.Printf("✅ เชื่อมต่อ Telegram Bot สำเร็จ!\nชื่อบอท: %s (@%s)\nBot ID: %d\n", user.FirstName, user.Username, user.ID)
		return
	}

	_ = initDB(*mockFlag)

	if *checkFlag {
		d := getDB()
		res := d.TestConnection()
		fmt.Printf("ผลการตรวจสอบฐานข้อมูล:\n%+v\n", res)
		return
	}

	if *nowFlag || *dryRunFlag || *dateFlag != "" {
		targetDay := time.Now().In(db.BangkokLoc)
		if *dateFlag != "" {
			parsed, err := time.ParseInLocation("2006-01-02", *dateFlag, db.BangkokLoc)
			if err == nil {
				targetDay = parsed
			}
		}
		_ = runOnce(targetDay, *dryRunFlag, true)
		return
	}

	// สร้าง Web Server พร้อม callback เชื่อมต่อ DB ใหม่อัตโนมัติเมื่อบันทึกตั้งค่า
	webInstance = web.NewWebServer(
		getDB,
		func() error {
			return initDB(false)
		},
		func() error {
			return runOnce(time.Now(), false, true)
		},
		func() error {
			return runTelegramOnce(time.Now(), true)
		},
	)

	cfg := config.LoadSettings()
	port := cfg.WebPort
	if port == 0 {
		port = 8090
	}

	// เริ่ม Background Scheduler หลัก (ใช้ร่วมกันทั้ง LINE และ Telegram)
	go startScheduler()

	// เริ่ม Telegram Poller และ Realtime Watcher
	tgPoller = telegram.NewPoller(cfg.EffectiveTelegramToken())
	tgPoller.Start(context.Background())

	tgWatcher = telegram.NewRealtimeWatcher(getDB)
	tgWatcher.Start(context.Background())

	// เริ่ม Web Server (Blocking)
	webInstance.Start(port)
}
