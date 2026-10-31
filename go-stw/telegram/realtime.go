package telegram

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"stw_teacher_check/attendance"
	"stw_teacher_check/config"
	"stw_teacher_check/db"
	"stw_teacher_check/logger"
	"stw_teacher_check/storage"
)

type ScanEvent struct {
	Enroll    int
	Name      string
	ScanTime  time.Time
	ScanIndex int    // 1 = สแกนแรก, 2 = สแกนครั้งที่สอง ...
	Status    string // "present" (ตรงเวลา), "late" (สาย), "repeat"
	LateMins  int
}

// DiffScans เปรียบเทียบข้อมูลสแกนใน DB กับประวัติที่เคยส่ง เพื่อหากิจกรรมการสแกนใหม่
// ตัดสแกนซ้ำของบุคคลเดิมที่ห่างจากสแกนก่อนหน้าน้อยกว่า 2 นาที
func DiffScans(
	people []attendance.Person,
	scans map[int][]time.Time,
	summary attendance.Summary,
	sentStore *SentStore,
	dedupeWindow time.Duration,
) []ScanEvent {
	personMap := make(map[int]attendance.Person)
	for _, p := range people {
		personMap[p.Enroll] = p
	}

	resultMap := make(map[int]attendance.Result)
	for _, r := range summary.Results {
		resultMap[r.Person.Enroll] = r
	}

	var events []ScanEvent

	for enroll, times := range scans {
		if len(times) == 0 {
			continue
		}
		// เรียงลำดับเวลาสแกนจากเช้าไปเย็น
		sorted := make([]time.Time, len(times))
		copy(sorted, times)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

		res, hasRes := resultMap[enroll]
		name := strings.TrimSpace(res.Person.Name)
		if !hasRes || name == "" {
			if p, hasP := personMap[enroll]; hasP {
				name = strings.TrimSpace(p.Name)
			}
		}
		if name == "" {
			name = fmt.Sprintf("รหัส %d", enroll)
		}

		var lastAccepted time.Time

		for idx, t := range sorted {
			// 1. ตรวจสอบว่าเคยส่งไปแล้วหรือยัง
			if sentStore.IsSent(enroll, t) {
				lastAccepted = t
				continue
			}

			// 2. ตรวจสอบการสแกนซ้ำภายในช่วงเวลา (2 นาที)
			if !lastAccepted.IsZero() && t.Sub(lastAccepted) < dedupeWindow {
				// Mark ว่าไม่ต้องส่ง แต่บันทึกว่าผ่านไปแล้วเพื่อไม่ให้ค้าง
				sentStore.MarkSent(enroll, t)
				continue
			}

			// สแกนที่ผ่านเกณฑ์
			lastAccepted = t

			ev := ScanEvent{
				Enroll:    enroll,
				Name:      name,
				ScanTime:  t,
				ScanIndex: idx + 1,
			}

			if idx == 0 {
				if res.Status == "late" {
					ev.Status = "late"
					ev.LateMins = res.LateMinutes
				} else {
					ev.Status = "present"
				}
			} else {
				ev.Status = "repeat"
			}

			events = append(events, ev)
		}
	}

	// เรียง Event ตามลำดับเวลาสแกนจริง
	sort.Slice(events, func(i, j int) bool {
		return events[i].ScanTime.Before(events[j].ScanTime)
	})

	return events
}

// FormatScanLine สร้างบรรทัดข้อความของแต่ละสแกน พร้อมเวลาจริงถึงระดับวินาทีจาก DB
// ✅ ชื่อ — 07:12:34 น. (ตรงเวลา)
// ⏰ ชื่อ — 07:45:10 น. (สาย 15 นาที)
// 🔄 ชื่อ — 16:30:05 น. (สแกนครั้งที่ 2)
func FormatScanLine(ev ScanEvent) string {
	timeStr := ev.ScanTime.Format("15:04:05")
	switch ev.Status {
	case "present":
		return fmt.Sprintf("✅ %s — %s น. (ตรงเวลา)", ev.Name, timeStr)
	case "late":
		return fmt.Sprintf("⏰ %s — %s น. (สาย %d นาที)", ev.Name, timeStr, ev.LateMins)
	default:
		return fmt.Sprintf("🔄 %s — %s น. (สแกนครั้งที่ %d)", ev.Name, timeStr, ev.ScanIndex)
	}
}

// FormatEvents จัดกลุ่มข้อความ: ถ้าน้อยกว่าหรือเท่ากับ 5 ส่งแยกข้อความละสแกน, ถ้ามากกว่า 5 รวมเป็นข้อความเดียว
func FormatEvents(events []ScanEvent) []string {
	if len(events) == 0 {
		return nil
	}

	if len(events) <= 5 {
		var msgs []string
		for _, ev := range events {
			msgs = append(msgs, FormatScanLine(ev))
		}
		return msgs
	}

	// มากกว่า 5 คน: รวมเป็นข้อความเดียว เรียงตามเวลา
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔔 รายการสแกนนิ้วล่าสุด (%d รายการ):\n", len(events)))
	for _, ev := range events {
		sb.WriteString(FormatScanLine(ev))
		sb.WriteString("\n")
	}
	return []string{strings.TrimRight(sb.String(), "\n")}
}

type RealtimeWatcher struct {
	dbSupplier func() db.Database
	sentStore  *SentStore
	client     *Client
	cancel     context.CancelFunc
}

func NewRealtimeWatcher(dbSupplier func() db.Database) *RealtimeWatcher {
	return &RealtimeWatcher{
		dbSupplier: dbSupplier,
		sentStore:  NewSentStore(""),
	}
}

func (w *RealtimeWatcher) Start(ctx context.Context) {
	watchCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	go func() {
		logger.Info("REALTIME", "🕒 เริ่มต้นระบบ Realtime Watcher ตรวจจับการสแกนนิ้วทุก 10 วินาที")
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-watchCtx.Done():
				logger.Info("REALTIME", "🛑 หยุดการทำงาน Realtime Watcher")
				return
			case <-ticker.C:
				w.checkOnce(watchCtx)
			}
		}
	}()
}

func (w *RealtimeWatcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}

func (w *RealtimeWatcher) checkOnce(ctx context.Context) {
	cfg := config.LoadSettings()
	token := cfg.EffectiveTelegramToken()
	if !cfg.TelegramEnabled || !cfg.TelegramRealtimeEnabled || token == "" {
		return
	}

	targets := storage.GetTelegramRealtimeIDs()
	if len(targets) == 0 {
		return
	}

	database := w.dbSupplier()
	if database == nil {
		return
	}

	now := time.Now().In(db.BangkokLoc)
	people, err := database.GetPeople()
	if err != nil {
		return
	}
	scans, err := database.GetScans(now)
	if err != nil {
		return
	}

	shifts, _ := database.GetShifts()
	dailyShift, _ := database.GetDailyShift(now)
	leaves, _ := database.GetLeaves(now)
	holidays, _ := database.GetHolidays()
	_, workdays, _ := database.GetSchool()

	summary := attendance.Compute(
		now, people, scans, shifts, dailyShift, leaves, holidays, workdays,
		cfg.LateTime, now,
	)

	events := DiffScans(people, scans, summary, w.sentStore, 2*time.Minute)
	if len(events) == 0 {
		return
	}

	if w.client == nil || w.client.token != token {
		w.client = NewClient(token)
	}

	messages := FormatEvents(events)
	for _, text := range messages {
		for _, chatID := range targets {
			_ = w.client.SendMessage(ctx, chatID, text)
		}
	}

	// บันทึกสถานะว่าส่งสำเร็จแล้ว
	w.sentStore.MarkBatchSent(events)
	logger.Info("REALTIME", "⚡ ส่งแจ้งเตือนสแกนนิ้วเข้า Telegram แล้ว %d รายการ (%d ข้อความ)", len(events), len(messages))
}

