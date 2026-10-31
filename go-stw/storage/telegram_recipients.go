package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TelegramRecipient struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "private", "group", "supergroup", "channel"
	Username    string `json:"username,omitempty"`
	RealtimeOn  bool   `json:"realtime_on"`
	SummaryOn   bool   `json:"summary_on"`
	CreatedAt   string `json:"created_at"`
	LastSeen    string `json:"last_seen,omitempty"`
}

type TelegramDiscovered struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Username  string `json:"username,omitempty"`
	LastSeen  string `json:"last_seen"`
}

var (
	tgStorageMu       sync.RWMutex
	tgRecipientsFile = filepath.Join("data", "telegram_recipients.json")
	tgDiscoveredFile = filepath.Join("data", "telegram_discovered.json")
)

func GetTelegramRecipients() []TelegramRecipient {
	tgStorageMu.RLock()
	defer tgStorageMu.RUnlock()
	return readTelegramRecipientsUnsafe()
}

func GetTelegramRealtimeIDs() []int64 {
	tgStorageMu.RLock()
	defer tgStorageMu.RUnlock()
	list := readTelegramRecipientsUnsafe()
	var ids []int64
	for _, r := range list {
		if r.RealtimeOn {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func GetTelegramSummaryIDs() []int64 {
	tgStorageMu.RLock()
	defer tgStorageMu.RUnlock()
	list := readTelegramRecipientsUnsafe()
	var ids []int64
	for _, r := range list {
		if r.SummaryOn {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func UpsertTelegramUser(id int64, name, chatType, username string, autoAdd bool) (TelegramRecipient, bool) {
	tgStorageMu.Lock()
	defer tgStorageMu.Unlock()

	nowStr := time.Now().Format("2006-01-02 15:04:05")
	list := readTelegramRecipientsUnsafe()

	for i, r := range list {
		if r.ID == id {
			if name != "" {
				list[i].Name = name
			}
			if username != "" {
				list[i].Username = username
			}
			list[i].LastSeen = nowStr
			_ = writeTelegramRecipientsUnsafe(list)
			return list[i], false
		}
	}

	if autoAdd {
		newRec := TelegramRecipient{
			ID:         id,
			Name:       name,
			Type:       chatType,
			Username:   username,
			RealtimeOn: true,
			SummaryOn:  true,
			CreatedAt:  nowStr,
			LastSeen:   nowStr,
		}
		list = append(list, newRec)
		_ = writeTelegramRecipientsUnsafe(list)
		removeTelegramDiscoveredUnsafe(id)
		return newRec, true
	}

	recordTelegramDiscoveredUnsafe(id, name, chatType, username)
	return TelegramRecipient{ID: id, Name: name, Type: chatType, Username: username}, true
}

func ToggleTelegramRecipient(id int64, field string, state bool) bool {
	tgStorageMu.Lock()
	defer tgStorageMu.Unlock()

	list := readTelegramRecipientsUnsafe()
	found := false
	for i, r := range list {
		if r.ID == id {
			if field == "realtime" {
				list[i].RealtimeOn = state
			} else if field == "summary" {
				list[i].SummaryOn = state
			}
			found = true
			break
		}
	}
	if found {
		_ = writeTelegramRecipientsUnsafe(list)
	}
	return found
}

func RemoveTelegramRecipient(id int64) bool {
	tgStorageMu.Lock()
	defer tgStorageMu.Unlock()

	list := readTelegramRecipientsUnsafe()
	var out []TelegramRecipient
	found := false
	for _, r := range list {
		if r.ID == id {
			found = true
			continue
		}
		out = append(out, r)
	}
	if found {
		_ = writeTelegramRecipientsUnsafe(out)
	}
	return found
}

func readTelegramRecipientsUnsafe() []TelegramRecipient {
	b, err := os.ReadFile(tgRecipientsFile)
	if err != nil {
		return []TelegramRecipient{}
	}
	var list []TelegramRecipient
	_ = json.Unmarshal(b, &list)
	return list
}

func writeTelegramRecipientsUnsafe(list []TelegramRecipient) error {
	_ = os.MkdirAll("data", 0755)
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := tgRecipientsFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, tgRecipientsFile)
}

func GetTelegramDiscovered() []TelegramDiscovered {
	tgStorageMu.RLock()
	defer tgStorageMu.RUnlock()
	return readTelegramDiscoveredUnsafe()
}

func recordTelegramDiscoveredUnsafe(id int64, name, chatType, username string) {
	list := readTelegramDiscoveredUnsafe()
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	for i, d := range list {
		if d.ID == id {
			list[i].Name = name
			list[i].LastSeen = nowStr
			if username != "" {
				list[i].Username = username
			}
			_ = writeTelegramDiscoveredUnsafe(list)
			return
		}
	}
	list = append(list, TelegramDiscovered{
		ID:       id,
		Name:     name,
		Type:     chatType,
		Username: username,
		LastSeen: nowStr,
	})
	_ = writeTelegramDiscoveredUnsafe(list)
}

func removeTelegramDiscoveredUnsafe(id int64) bool {
	list := readTelegramDiscoveredUnsafe()
	var out []TelegramDiscovered
	found := false
	for _, d := range list {
		if d.ID == id {
			found = true
			continue
		}
		out = append(out, d)
	}
	if found {
		_ = writeTelegramDiscoveredUnsafe(out)
	}
	return found
}

func readTelegramDiscoveredUnsafe() []TelegramDiscovered {
	b, err := os.ReadFile(tgDiscoveredFile)
	if err != nil {
		return []TelegramDiscovered{}
	}
	var list []TelegramDiscovered
	_ = json.Unmarshal(b, &list)
	return list
}

func writeTelegramDiscoveredUnsafe(list []TelegramDiscovered) error {
	_ = os.MkdirAll("data", 0755)
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := tgDiscoveredFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, tgDiscoveredFile)
}

func AddTelegramRecipientDirect(id int64, name string) (TelegramRecipient, error) {
	tgStorageMu.Lock()
	defer tgStorageMu.Unlock()

	if id == 0 {
		return TelegramRecipient{}, fmt.Errorf("invalid chat id")
	}

	list := readTelegramRecipientsUnsafe()
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	for _, r := range list {
		if r.ID == id {
			return r, nil
		}
	}

	newRec := TelegramRecipient{
		ID:         id,
		Name:       name,
		Type:       "manual",
		RealtimeOn: true,
		SummaryOn:  true,
		CreatedAt:  nowStr,
		LastSeen:   nowStr,
	}
	list = append(list, newRec)
	_ = writeTelegramRecipientsUnsafe(list)
	removeTelegramDiscoveredUnsafe(id)
	return newRec, nil
}

