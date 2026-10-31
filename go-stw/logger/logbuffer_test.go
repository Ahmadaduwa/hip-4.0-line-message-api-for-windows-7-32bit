package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailyLoggerAndRetention(t *testing.T) {
	testDir := "test_logs"
	_ = os.RemoveAll(testDir)
	defer os.RemoveAll(testDir)

	now := time.Now().In(BangkokLoc)

	l := &DailyLogger{
		logsDir:       testDir,
		retentionDays: 30,
		nowFunc:       func() time.Time { return now },
	}
	defer l.Close()

	l.rotateAndCleanUnsafe(now)

	// Create a log file that is 35 days old (should be cleaned up)
	oldDate := now.AddDate(0, 0, -35).Format("2006-01-02")
	oldFile := filepath.Join(testDir, fmt.Sprintf("app-%s.log", oldDate))
	_ = os.WriteFile(oldFile, []byte("old log"), 0644)

	// Create a log file that is 10 days old (should be kept)
	recentDate := now.AddDate(0, 0, -10).Format("2006-01-02")
	recentFile := filepath.Join(testDir, fmt.Sprintf("app-%s.log", recentDate))
	_ = os.WriteFile(recentFile, []byte("recent log"), 0644)

	// Write log using DailyLogger
	l.WriteLog("INFO", "TEST", "ทดสอบบันทึก Log", "")
	l.WriteLog("ERROR", "DB", "เชื่อมต่อไม่สำเร็จ", "details error trace")

	// Trigger cleanup
	l.cleanOldLogsUnsafe(now)

	// Verify old file was deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old log file %s to be deleted by 30-day retention", oldFile)
	}

	// Verify recent file is kept
	if _, err := os.Stat(recentFile); err != nil {
		t.Errorf("expected recent log file %s to exist", recentFile)
	}

	// Verify today's file exists and has content
	todayFile := filepath.Join(testDir, fmt.Sprintf("app-%s.log", now.Format("2006-01-02")))
	content, err := os.ReadFile(todayFile)
	if err != nil {
		t.Fatalf("failed to read today log file: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "[INFO ] [TEST] ทดสอบบันทึก Log") {
		t.Errorf("missing INFO log content in today file: %s", contentStr)
	}
	if !strings.Contains(contentStr, "[ERROR] [DB] เชื่อมต่อไม่สำเร็จ") || !strings.Contains(contentStr, "↳ Error: details error trace") {
		t.Errorf("missing ERROR details in today file: %s", contentStr)
	}
}
