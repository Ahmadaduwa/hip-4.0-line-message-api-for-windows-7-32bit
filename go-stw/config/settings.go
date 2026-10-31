package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Settings struct {
	LateTime          string   `json:"late_time"`
	AutoDownloadTime  string   `json:"auto_download_time"`
	SendTimes         []string `json:"send_times"`
	LateGraceMinutes  int      `json:"late_grace_minutes"`
	DBHost            string   `json:"db_host"`
	DBPort            int      `json:"db_port"`
	DBName            string   `json:"db_name"`
	DBUser            string   `json:"db_user"`
	DBPassword        string   `json:"db_password,omitempty"`
	DBAuth            string   `json:"db_auth"` // "windows", "sql", "mock"
	CompanyName       string   `json:"company_name"`
	ShowPresent       bool     `json:"show_present"`
	SendOnDayoff      bool     `json:"send_on_dayoff"`
	MockMode          bool     `json:"mock_mode"`
	WebPort           int      `json:"web_port"`
	WebUsername       string   `json:"web_username"`
	WebPassword       string   `json:"web_password"`
	LineChannelToken  string   `json:"line_channel_access_token,omitempty"`
	LineChannelSecret string   `json:"line_channel_secret,omitempty"`
	AutoAddRecipients bool     `json:"auto_add_recipients"`
	WelcomeMessage    string   `json:"welcome_message,omitempty"`

	// ---------- Telegram ----------
	TelegramToken           string   `json:"telegram_token,omitempty"` // ค่าจากหน้าเว็บ (มาก่อน .env)
	TelegramEnabled         bool     `json:"telegram_enabled"`
	TelegramAutoAdd         bool     `json:"telegram_auto_add"`
	TelegramRealtimeEnabled bool     `json:"telegram_realtime_enabled"`
	TelegramSummaryEnabled  bool     `json:"telegram_summary_enabled"`
	TelegramShowPresent     bool     `json:"telegram_show_present"`
	TelegramSendTimes       []string `json:"telegram_send_times"`

	// TelegramTokenEnv คือ TELEGRAM_TOKEN จาก .env (ไม่บันทึกลง settings.json)
	TelegramTokenEnv string `json:"-"`
}

// EffectiveTelegramToken คืน Token ที่ใช้งานจริง: ค่าจากหน้าเว็บก่อน ถ้าว่างใช้ TELEGRAM_TOKEN ใน .env
func (s Settings) EffectiveTelegramToken() string {
	if t := strings.TrimSpace(s.TelegramToken); t != "" {
		return t
	}
	return strings.TrimSpace(s.TelegramTokenEnv)
}

// NormalizeTimes ตรวจรูปแบบเวลา (H:MM / HH:MM / HH.MM) แปลงเป็น HH:MM ตัดค่าซ้ำ และเรียงจากเช้าไปเย็น
func NormalizeTimes(in []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(in))
	for _, s := range in {
		m := hmRegex.FindStringSubmatch(strings.TrimSpace(s))
		if m == nil {
			continue
		}
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		if h > 23 || mi > 59 {
			continue
		}
		t := fmt.Sprintf("%02d:%02d", h, mi)
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

var hmRegex = regexp.MustCompile(`^(\d{1,2})[:.](\d{2})$`)

// applyEnvDefaults ใช้ค่าจาก .env เป็นค่าเริ่มต้นก่อนอ่าน settings.json (ค่าในไฟล์/หน้าเว็บจะทับได้)
func applyEnvDefaults(cfg *Settings) {
	env := loadEnvMap()
	if times := NormalizeTimes(strings.Split(env["SEND_TIMES"], ",")); len(times) > 0 {
		cfg.SendTimes = times
	}
}

// normalizeSchedules ใช้ร่วมกันตอนโหลดและบันทึก
func normalizeSchedules(cfg *Settings) {
	cfg.SendTimes = NormalizeTimes(cfg.SendTimes)
	if len(cfg.SendTimes) == 0 {
		cfg.SendTimes = []string{"09:05"}
	}
	cfg.TelegramSendTimes = NormalizeTimes(cfg.TelegramSendTimes)
	if len(cfg.TelegramSendTimes) == 0 {
		cfg.TelegramSendTimes = []string{"09:05"}
	}
}

var (
	settingsLock sync.RWMutex
	current      *Settings
	dataDir      = "data"
	settingsFile = filepath.Join("data", "settings.json")
)

func DefaultSettings() Settings {
	return Settings{
		LateTime:          "07:30",
		AutoDownloadTime:  "09:00",
		SendTimes:         []string{"09:05"},
		LateGraceMinutes:  0,
		DBHost:            "127.0.0.1",
		DBPort:            1433,
		DBName:            "hiptime",
		DBUser:            "sa",
		DBPassword:        "",
		DBAuth:            "sql",
		CompanyName:       "โรงเรียนแสงธรรมวิทยา",
		ShowPresent:       true,
		SendOnDayoff:      false,
		MockMode:          false,
		WebPort:           8090,
		WebUsername:       "admin",
		WebPassword:       "admin1234",
		LineChannelToken:  "",
		LineChannelSecret: "",
		AutoAddRecipients: true,
		WelcomeMessage:    "",

		TelegramEnabled:         true,
		TelegramAutoAdd:         true,
		TelegramRealtimeEnabled: true,
		TelegramSummaryEnabled:  true,
		TelegramShowPresent:     true,
		TelegramSendTimes:       []string{"09:05"},
	}
}

func loadEnvMap() map[string]string {
	envMap := make(map[string]string)
	candidates := []string{".env", filepath.Join("..", ".env")}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, ".env"),
			filepath.Join(exeDir, "..", ".env"),
		)
	}
	for _, p := range candidates {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				if _, exists := envMap[k]; !exists && v != "" {
					envMap[k] = v
				}
			}
		}
		_ = f.Close()
	}
	return envMap
}

func applyEnvFallbacks(cfg *Settings) {
	env := loadEnvMap()
	if cfg.LineChannelToken == "" {
		if tok := env["LINE_CHANNEL_ACCESS_TOKEN"]; tok != "" {
			cfg.LineChannelToken = tok
		}
	}
	if cfg.LineChannelSecret == "" {
		if sec := env["LINE_CHANNEL_SECRET"]; sec != "" {
			cfg.LineChannelSecret = sec
		}
	}
	if pw := env["DB_PASSWORD"]; pw != "" {
		cfg.DBPassword = pw
		if cfg.DBAuth == "" || cfg.DBAuth == "windows" {
			cfg.DBAuth = "sql"
		}
	}
	if v := env["DB_HOST"]; v != "" && (cfg.DBHost == "" || cfg.DBHost == "127.0.0.1") {
		cfg.DBHost = v
	}
	if v := env["DB_NAME"]; v != "" {
		cfg.DBName = v
	}
	if v := env["COMPANY_NAME"]; v != "" {
		cfg.CompanyName = v
	}
	// SEND_TIMES ใน .env ใช้เป็นค่าเริ่มต้นเท่านั้น (ดู applyEnvDefaults) เพื่อให้ค่าที่ตั้งจากหน้าเว็บมีผลเสมอ
	if len(cfg.SendTimes) == 0 {
		cfg.SendTimes = NormalizeTimes(strings.Split(env["SEND_TIMES"], ","))
	}
	cfg.TelegramTokenEnv = env["TELEGRAM_TOKEN"]
	if v := env["WEB_PORT"]; v != "" && cfg.WebPort == 0 {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.WebPort = p
		}
	}
}

func LoadSettings() Settings {
	settingsLock.Lock()
	defer settingsLock.Unlock()

	if current != nil {
		return *current
	}

	_ = os.MkdirAll(dataDir, 0755)
	defaults := DefaultSettings()
	applyEnvDefaults(&defaults)
	applyEnvFallbacks(&defaults)

	bytes, err := os.ReadFile(settingsFile)
	if err != nil {
		current = &defaults
		_ = saveToFile(defaults)
		return defaults
	}

	cfg := defaults
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		current = &defaults
		return defaults
	}
	applyEnvFallbacks(&cfg)

	normalizeSchedules(&cfg)
	if cfg.LateTime == "" {
		cfg.LateTime = "07:30"
	}
	if cfg.DBName == "" {
		cfg.DBName = "Synctime"
	}
	if cfg.CompanyName == "" {
		cfg.CompanyName = "โรงเรียนแสงธรรมวิทยา"
	}
	if cfg.DBHost == "" {
		cfg.DBHost = "127.0.0.1"
	}
	if cfg.DBAuth == "" {
		if cfg.DBPassword != "" {
			cfg.DBAuth = "sql"
		} else {
			cfg.DBAuth = "windows"
		}
	}
	if cfg.WebPort == 0 {
		cfg.WebPort = 8090
	}
	if cfg.WebUsername == "" {
		cfg.WebUsername = "admin"
	}
	if cfg.WebPassword == "" || cfg.WebPassword == "change_this_password" {
		cfg.WebPassword = "admin1234"
	}
	applyEnvFallbacks(&cfg)
	_ = saveToFile(cfg)

	current = &cfg
	return *current
}

func SaveSettings(newCfg Settings) (Settings, error) {
	settingsLock.Lock()
	defer settingsLock.Unlock()

	normalizeSchedules(&newCfg)
	if newCfg.LateTime == "" {
		newCfg.LateTime = "07:30"
	}
	if newCfg.DBName == "" {
		newCfg.DBName = "Synctime"
	}
	if newCfg.CompanyName == "" {
		newCfg.CompanyName = "โรงเรียนแสงธรรมวิทยา"
	}
	if newCfg.WebPort == 0 {
		newCfg.WebPort = 8090
	}
	if newCfg.WebUsername == "" {
		newCfg.WebUsername = "admin"
	}
	if newCfg.WebPassword == "" {
		newCfg.WebPassword = "admin1234"
	}
	applyEnvFallbacks(&newCfg)

	current = &newCfg
	err := saveToFile(newCfg)
	return newCfg, err
}

func saveToFile(cfg Settings) error {
	_ = os.MkdirAll(dataDir, 0755)
	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := settingsFile + ".tmp"
	if err := os.WriteFile(tmpFile, bytes, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, settingsFile)
}
