package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stw_teacher_check/config"
	"stw_teacher_check/logger"
	"stw_teacher_check/storage"
)

type Poller struct {
	client     *Client
	token      string
	cancel     context.CancelFunc
	lastOffset int64
}

func NewPoller(token string) *Poller {
	return &Poller{
		token:  token,
		client: NewClient(token),
	}
}

func (p *Poller) Start(ctx context.Context) {
	pollCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	go func() {
		logger.Info("TELEGRAM", "🚀 เริ่มต้น Telegram Bot Poller (Long Polling)")
		for {
			select {
			case <-pollCtx.Done():
				logger.Info("TELEGRAM", "🛑 หยุดการทำงาน Telegram Poller")
				return
			default:
			}

			cfg := config.LoadSettings()
			token := cfg.EffectiveTelegramToken()
			if !cfg.TelegramEnabled || token == "" {
				time.Sleep(3 * time.Second)
				continue
			}

			if p.token != token {
				p.token = token
				p.client = NewClient(token)
			}

			// ดึง Updates ด้วย timeout 15 วินาที
			reqCtx, reqCancel := context.WithTimeout(pollCtx, 25*time.Second)
			updates, err := p.client.GetUpdates(reqCtx, p.lastOffset, 15)
			reqCancel()

			if err != nil {
				if pollCtx.Err() != nil {
					return
				}
				logger.Warn("TELEGRAM", "⚠️ ดึง Updates จาก Telegram ไม่สำเร็จ: %v (จะลองใหม่ใน 5 วินาที)", err)
				time.Sleep(5 * time.Second)
				continue
			}

			for _, u := range updates {
				if u.UpdateID >= p.lastOffset {
					p.lastOffset = u.UpdateID + 1
				}
				p.handleUpdate(pollCtx, u, cfg)
			}
		}
	}()
}

func (p *Poller) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *Poller) handleUpdate(ctx context.Context, u Update, cfg config.Settings) {
	// 1. กรณีข้อความ (/start)
	if u.Message != nil && u.Message.Chat.ID != 0 {
		chatID := u.Message.Chat.ID
		chatType := u.Message.Chat.Type
		text := strings.TrimSpace(u.Message.Text)

		name := ""
		username := ""
		if u.Message.From != nil {
			name = strings.TrimSpace(u.Message.From.FirstName + " " + u.Message.From.LastName)
			username = u.Message.From.Username
		}
		if name == "" {
			name = u.Message.Chat.Title
		}
		if name == "" {
			name = fmt.Sprintf("Chat_%d", chatID)
		}

		if strings.HasPrefix(text, "/start") {
			rec, isNew := storage.UpsertTelegramUser(chatID, name, chatType, username, cfg.TelegramAutoAdd)
			if isNew {
				logger.Info("TELEGRAM", "➕ ได้รับสมาชิก Telegram ใหม่: %s (Chat ID: %d, Type: %s)", rec.Name, rec.ID, rec.Type)
			} else {
				logger.Info("TELEGRAM", "💬 สมาชิก Telegram ส่ง /start ซ้ำ: %s (Chat ID: %d)", rec.Name, rec.ID)
			}

			// ส่งข้อความต้อนรับ
			welcome := ""
			if cfg.TelegramAutoAdd {
				welcome = fmt.Sprintf("👋 สวัสดีครับคุณ %s\nลงทะเบียนรับการแจ้งเตือนจากระบบ STW_TeacherCheck สำเร็จ!\n\n🔔 สถานะการรับข้อความ:\n• แจ้งเตือนสแกนนิ้ว Realtime: เปิด [ON]\n• รายงานสรุปประจำวัน: เปิด [ON] (เวลา %s น.)", rec.Name, strings.Join(cfg.SendTimes, ", "))
			} else {
				welcome = fmt.Sprintf("👋 สวัสดีครับคุณ %s\nระบบบันทึกคำขอของคุณแล้ว กรุณาแจ้งผู้ดูแลระบบเพื่อเปิดสวิตช์การรับข้อความในหน้าเว็บก่อนครับ\n(Chat ID: %d)", rec.Name, rec.ID)
			}
			_ = p.client.SendMessage(ctx, chatID, welcome)
		}
	}

	// 2. กรณีถูกเพิ่มเข้ากลุ่ม (my_chat_member)
	if u.MyChatMember != nil && u.MyChatMember.Chat.ID != 0 {
		chatID := u.MyChatMember.Chat.ID
		chatType := u.MyChatMember.Chat.Type
		title := u.MyChatMember.Chat.Title
		if title == "" {
			title = fmt.Sprintf("Group_%d", chatID)
		}

		status := u.MyChatMember.NewChatMember.Status
		if status == "member" || status == "administrator" {
			rec, _ := storage.UpsertTelegramUser(chatID, title, chatType, "", cfg.TelegramAutoAdd)
			logger.Info("TELEGRAM", "👥 บอทถูกเพิ่มเข้ากลุ่ม Telegram: %s (ID: %d)", rec.Name, rec.ID)
			welcome := fmt.Sprintf("👋 สวัสดีครับกลุ่ม %s\nบอท STW_TeacherCheck พร้อมทำงานแล้วครับ", title)
			_ = p.client.SendMessage(ctx, chatID, welcome)
		} else if status == "left" || status == "kicked" {
			storage.ToggleTelegramRecipient(chatID, "realtime", false)
			storage.ToggleTelegramRecipient(chatID, "summary", false)
			logger.Info("TELEGRAM", "🚪 บอทถูกนำออกจากกลุ่ม Telegram: %s (ID: %d)", title, chatID)
		}
	}
}

