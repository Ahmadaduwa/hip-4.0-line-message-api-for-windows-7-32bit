package telegram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type SentState struct {
	Date string          `json:"date"` // YYYY-MM-DD
	Sent map[string]bool `json:"sent"` // key: "enroll|timestamp_RFC3339"
}

type SentStore struct {
	mu       sync.Mutex
	filePath string
	state    SentState
}

func NewSentStore(filePath string) *SentStore {
	if filePath == "" {
		filePath = filepath.Join("data", "telegram_sent.json")
	}
	s := &SentStore{filePath: filePath}
	s.load()
	return s
}

func (s *SentStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	b, err := os.ReadFile(s.filePath)
	if err == nil {
		var state SentState
		if err := json.Unmarshal(b, &state); err == nil && state.Date == today {
			s.state = state
			if s.state.Sent == nil {
				s.state.Sent = make(map[string]bool)
			}
			return
		}
	}

	// ถ้าไม่มีไฟล์ หรือเป็นของวันเก่า ให้เริ่ม state ใหม่ของวันนี้
	s.state = SentState{
		Date: today,
		Sent: make(map[string]bool),
	}
	_ = s.saveUnsafe()
}

func (s *SentStore) IsSent(enroll int, t time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := t.Format("2006-01-02")
	if s.state.Date != today {
		s.state.Date = today
		s.state.Sent = make(map[string]bool)
	}

	key := makeScanKey(enroll, t)
	return s.state.Sent[key]
}

func (s *SentStore) MarkSent(enroll int, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := t.Format("2006-01-02")
	if s.state.Date != today {
		s.state.Date = today
		s.state.Sent = make(map[string]bool)
	}

	key := makeScanKey(enroll, t)
	s.state.Sent[key] = true
	_ = s.saveUnsafe()
}

func (s *SentStore) MarkBatchSent(events []ScanEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ev := range events {
		today := ev.ScanTime.Format("2006-01-02")
		if s.state.Date != today {
			s.state.Date = today
			s.state.Sent = make(map[string]bool)
		}
		key := makeScanKey(ev.Enroll, ev.ScanTime)
		s.state.Sent[key] = true
	}
	_ = s.saveUnsafe()
}

func (s *SentStore) saveUnsafe() error {
	_ = os.MkdirAll(filepath.Dir(s.filePath), 0755)
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

func makeScanKey(enroll int, t time.Time) string {
	return fmt.Sprintf("%s_%d", t.Format("20060102150405"), enroll)
}

