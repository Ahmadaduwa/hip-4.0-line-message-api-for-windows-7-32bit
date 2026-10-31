package telegram

import (
	"context"
	"fmt"

	"stw_teacher_check/attendance"
	"stw_teacher_check/logger"
	"stw_teacher_check/storage"
)

// DeliverSummary ส่งข้อความสรุปประจำวันเข้า Telegram ไปยังผู้รับทุกคนที่เปิด SummaryOn
func DeliverSummary(ctx context.Context, token string, summary attendance.Summary, text string) error {
	if token == "" {
		return fmt.Errorf("telegram token is empty")
	}

	targets := storage.GetTelegramSummaryIDs()
	if len(targets) == 0 {
		logger.Info("TELEGRAM", "ไม่มีผู้รับรายงานสรุป Telegram ที่เปิดสวิตช์ไว้")
		return nil
	}

	client := NewClient(token)
	successCount := 0
	for _, chatID := range targets {
		err := client.SendMessage(ctx, chatID, text)
		if err != nil {
			logger.Error("TELEGRAM", err, "ส่งสรุปเข้า Telegram (Chat ID: %d) ไม่สำเร็จ", chatID)
		} else {
			successCount++
		}
	}

	logger.Info("TELEGRAM", "📊 ส่งสรุปประจำวันเข้า Telegram สำเร็จ %d/%d ผู้รับ", successCount, len(targets))
	return nil
}
