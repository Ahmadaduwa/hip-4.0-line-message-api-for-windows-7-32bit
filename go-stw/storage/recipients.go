package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Recipient struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"` // "user" or "group"
	PictureURL string `json:"picture_url,omitempty"`
	Active     bool   `json:"active"`
	CreatedAt  string `json:"created_at"`
	LastSeen   string `json:"last_seen,omitempty"`
}

type DiscoveredTarget struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	PictureURL string `json:"picture_url,omitempty"`
	LastSeen   string `json:"last_seen"`
}

var (
	storageMu      sync.RWMutex
	recipientsFile = filepath.Join("data", "recipients.json")
	discoveredFile = filepath.Join("data", "discovered.json")
	targetIDRegex  = regexp.MustCompile(`^[UCR][a-f0-9]{32}$`)
)

func IsValidTargetID(id string) bool {
	return targetIDRegex.MatchString(strings.TrimSpace(id))
}

// ================== RECIPIENTS (UNIFIED LIST) ==================

func GetRecipients() []Recipient {
	storageMu.Lock()
	defer storageMu.Unlock()

	_ = os.MkdirAll("data", 0755)
	list := readRecipientsUnsafe()

	// รวมรายชื่อที่อาจค้างอยู่ใน discovered.json เข้ามาในรายการเดียว เพื่อให้เปิด/ปิดสวิตช์ได้ทันที
	disc := readDiscoveredUnsafe()
	if len(disc) > 0 {
		changed := false
		for _, d := range disc {
			exists := false
			for i := range list {
				if strings.EqualFold(list[i].ID, d.ID) {
					if list[i].PictureURL == "" && d.PictureURL != "" {
						list[i].PictureURL = d.PictureURL
						changed = true
					}
					if (list[i].Name == "" || list[i].Name == "ผู้ใช้งาน LINE" || list[i].Name == "ผู้รับรายงาน") && d.Name != "" {
						list[i].Name = d.Name
						changed = true
					}
					exists = true
					break
				}
			}
			if !exists && IsValidTargetID(d.ID) {
				tType := d.Type
				if tType == "" {
					if strings.HasPrefix(d.ID, "C") {
						tType = "group"
					} else {
						tType = "user"
					}
				}
				list = append(list, Recipient{
					ID:         d.ID,
					Name:       d.Name,
					Type:       tType,
					PictureURL: d.PictureURL,
					Active:     true,
					CreatedAt:  d.LastSeen,
					LastSeen:   d.LastSeen,
				})
				changed = true
			}
		}
		if changed {
			_ = writeRecipientsUnsafe(list)
		}
		_ = writeDiscoveredUnsafe([]DiscoveredTarget{})
	}

	return list
}

func GetActiveTargetIDs() []string {
	all := GetRecipients()
	var res []string
	for _, r := range all {
		if r.Active {
			res = append(res, r.ID)
		}
	}
	return res
}

func AddRecipient(id, name string) (Recipient, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if !IsValidTargetID(id) {
		return Recipient{}, fmt.Errorf("รูปแบบ LINE ID ไม่ถูกต้อง (ต้องขึ้นต้นด้วย U หรือ C และยาว 33 ตัวอักษร)")
	}

	storageMu.Lock()
	defer storageMu.Unlock()

	list := readRecipientsUnsafe()
	for i, r := range list {
		if strings.EqualFold(r.ID, id) {
			if name != "" {
				list[i].Name = name
				_ = writeRecipientsUnsafe(list)
			}
			return list[i], nil
		}
	}

	targetType := "user"
	if strings.HasPrefix(id, "C") {
		targetType = "group"
	}
	if name == "" {
		if targetType == "group" {
			name = "กลุ่ม LINE"
		} else {
			name = "ผู้รับรายงาน"
		}
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	newRec := Recipient{
		ID:        id,
		Name:      name,
		Type:      targetType,
		Active:    true,
		CreatedAt: now,
		LastSeen:  now,
	}
	list = append([]Recipient{newRec}, list...)
	_ = writeRecipientsUnsafe(list)
	removeDiscoveredUnsafe(id)
	return newRec, nil
}

// UpsertFromWebhook บันทึกคนที่แอดเพื่อนหรือทักแชทเข้ามาในรายการผู้รับทันที
// หากมีรายชื่ออยู่แล้ว จะคงค่าสถานะสวิตช์เปิด/ปิด (Active) เดิมไว้ตามที่แอดมินตั้งค่า
func UpsertFromWebhook(id, name, picURL string, defaultActive bool) (Recipient, bool) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if !IsValidTargetID(id) {
		return Recipient{}, false
	}

	storageMu.Lock()
	defer storageMu.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05")
	list := readRecipientsUnsafe()
	for i, r := range list {
		if strings.EqualFold(r.ID, id) {
			if name != "" && name != "ผู้ใช้งาน LINE" && name != "กลุ่ม LINE" {
				list[i].Name = name
			}
			if picURL != "" {
				list[i].PictureURL = picURL
			}
			list[i].LastSeen = now
			_ = writeRecipientsUnsafe(list)
			removeDiscoveredUnsafe(id)
			return list[i], false
		}
	}

	targetType := "user"
	if strings.HasPrefix(id, "C") {
		targetType = "group"
	}
	if name == "" {
		if targetType == "group" {
			name = "กลุ่ม LINE"
		} else {
			name = "ผู้ใช้งาน LINE"
		}
	}

	newRec := Recipient{
		ID:         id,
		Name:       name,
		Type:       targetType,
		PictureURL: picURL,
		Active:     defaultActive,
		CreatedAt:  now,
		LastSeen:   now,
	}
	list = append([]Recipient{newRec}, list...)
	_ = writeRecipientsUnsafe(list)
	removeDiscoveredUnsafe(id)
	return newRec, true
}

func AddOrActivateRecipient(id, name string) (Recipient, bool) {
	rec, isNew := UpsertFromWebhook(id, name, "", true)
	if !rec.Active {
		ToggleRecipient(id, true)
		rec.Active = true
	}
	return rec, isNew
}

func ToggleRecipient(id string, active bool) bool {
	storageMu.Lock()
	defer storageMu.Unlock()

	list := readRecipientsUnsafe()
	found := false
	for i := range list {
		if strings.EqualFold(list[i].ID, id) {
			list[i].Active = active
			found = true
			break
		}
	}
	if found {
		_ = writeRecipientsUnsafe(list)
	}
	return found
}

func RemoveRecipient(id string) bool {
	storageMu.Lock()
	defer storageMu.Unlock()

	list := readRecipientsUnsafe()
	var updated []Recipient
	found := false
	for _, r := range list {
		if strings.EqualFold(r.ID, id) {
			found = true
			continue
		}
		updated = append(updated, r)
	}
	if found {
		_ = writeRecipientsUnsafe(updated)
	}
	removeDiscoveredUnsafe(id)
	return found
}

func readRecipientsUnsafe() []Recipient {
	bytes, err := os.ReadFile(recipientsFile)
	if err != nil {
		return []Recipient{}
	}
	var list []Recipient
	_ = json.Unmarshal(bytes, &list)
	return list
}

func writeRecipientsUnsafe(list []Recipient) error {
	_ = os.MkdirAll("data", 0755)
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(recipientsFile, b, 0644)
}

// ================== DISCOVERED (LEGACY COMPAT) ==================

func GetDiscovered() []DiscoveredTarget {
	storageMu.RLock()
	defer storageMu.RUnlock()

	bytes, err := os.ReadFile(discoveredFile)
	if err != nil {
		return []DiscoveredTarget{}
	}
	var list []DiscoveredTarget
	_ = json.Unmarshal(bytes, &list)
	return list
}

func RecordDiscovered(id, name, targetType, picURL string) {
	UpsertFromWebhook(id, name, picURL, false)
}

func RemoveDiscovered(id string) bool {
	storageMu.Lock()
	defer storageMu.Unlock()
	return removeDiscoveredUnsafe(id)
}

func removeDiscoveredUnsafe(id string) bool {
	list := readDiscoveredUnsafe()
	var updated []DiscoveredTarget
	found := false
	for _, d := range list {
		if strings.EqualFold(d.ID, id) {
			found = true
			continue
		}
		updated = append(updated, d)
	}
	if found {
		_ = writeDiscoveredUnsafe(updated)
	}
	return found
}

func readDiscoveredUnsafe() []DiscoveredTarget {
	bytes, err := os.ReadFile(discoveredFile)
	if err != nil {
		return []DiscoveredTarget{}
	}
	var list []DiscoveredTarget
	_ = json.Unmarshal(bytes, &list)
	return list
}

func writeDiscoveredUnsafe(list []DiscoveredTarget) error {
	_ = os.MkdirAll("data", 0755)
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(discoveredFile, b, 0644)
}
