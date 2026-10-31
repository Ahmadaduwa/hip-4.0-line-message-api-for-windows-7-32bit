package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var BangkokLoc = time.FixedZone("Asia/Bangkok", 7*3600)

type LogEntry struct {
	Time         string `json:"time"`
	Level        string `json:"level"`
	Name         string `json:"name"`
	Message      string `json:"message"`
	ErrorDetails string `json:"error_details,omitempty"`
}

type DailyLogger struct {
	mu            sync.Mutex
	logsDir       string
	currentDay    string
	currentFile   *os.File
	retentionDays int
	nowFunc       func() time.Time
}

var (
	instance *DailyLogger
	initOnce sync.Once
)

func GetDailyLogger() *DailyLogger {
	initOnce.Do(func() {
		instance = &DailyLogger{
			logsDir:       "logs",
			retentionDays: 30,
		}
		_ = os.MkdirAll(instance.logsDir, 0755)
		instance.rotateAndCleanUnsafe(time.Now().In(BangkokLoc))
	})
	return instance
}

func InitLogger() {
	_ = GetDailyLogger()
}

func (l *DailyLogger) rotateAndCleanUnsafe(now time.Time) {
	today := now.Format("2006-01-02")
	if l.currentDay != today || l.currentFile == nil {
		if l.currentFile != nil {
			_ = l.currentFile.Close()
			l.currentFile = nil
		}
		_ = os.MkdirAll(l.logsDir, 0755)
		filePath := filepath.Join(l.logsDir, fmt.Sprintf("app-%s.log", today))
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			l.currentFile = f
			l.currentDay = today
		}
		l.cleanOldLogsUnsafe(now)
	}
}

func (l *DailyLogger) cleanOldLogsUnsafe(now time.Time) {
	cutoff := now.AddDate(0, 0, -l.retentionDays)
	entries, err := os.ReadDir(l.logsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// ค้นหาไฟล์รูปแบบ app-YYYY-MM-DD.log
		if strings.HasPrefix(name, "app-") && strings.HasSuffix(name, ".log") {
			datePart := strings.TrimSuffix(strings.TrimPrefix(name, "app-"), ".log")
			t, err := time.ParseInLocation("2006-01-02", datePart, BangkokLoc)
			if err == nil && t.Before(cutoff) {
				_ = os.Remove(filepath.Join(l.logsDir, name))
			}
		}
	}
}

func (l *DailyLogger) now() time.Time {
	if l.nowFunc != nil {
		return l.nowFunc().In(BangkokLoc)
	}
	return time.Now().In(BangkokLoc)
}

func (l *DailyLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.currentFile != nil {
		err := l.currentFile.Close()
		l.currentFile = nil
		return err
	}
	return nil
}

func (l *DailyLogger) WriteLog(level, name, msg, errDetails string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.rotateAndCleanUnsafe(now)

	timestamp := now.Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("%s [%-5s] [%s] %s\n", timestamp, level, name, msg)
	if errDetails != "" {
		line += fmt.Sprintf("   ↳ Error: %s\n", errDetails)
	}

	// พิมพ์ออก Console
	fmt.Print(line)

	// บันทึกลงไฟล์ประจำวัน
	if l.currentFile != nil {
		_, _ = l.currentFile.WriteString(line)
	}
}

func Log(level, name, msg, errDetails string) {
	GetDailyLogger().WriteLog(level, name, msg, errDetails)
}

func Info(name, format string, a ...interface{}) {
	Log("INFO", name, fmt.Sprintf(format, a...), "")
}

func Warning(name, format string, a ...interface{}) {
	Log("WARN", name, fmt.Sprintf(format, a...), "")
}

func Warn(name, format string, a ...interface{}) {
	Warning(name, format, a...)
}

func Error(name string, err error, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	var details string
	if err != nil {
		details = err.Error()
	}
	Log("ERROR", name, msg, details)
}

// RingBuffer dummy for backward compatibility
type RingBuffer struct{}

var GlobalBuffer = &RingBuffer{}

func (r *RingBuffer) GetLogs(limit int) []LogEntry { return nil }
func (r *RingBuffer) Clear()                       {}
