package config

import (
	"reflect"
	"testing"
)

func TestNormalizeTimes(t *testing.T) {
	got := NormalizeTimes([]string{" 16:30", "9:05", "09:05", "bad", "24:00", "12.00", "", "07:61"})
	want := []string{"09:05", "12:00", "16:30"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeTimes = %v, want %v", got, want)
	}
}

func TestNormalizeSchedulesFallback(t *testing.T) {
	cfg := Settings{SendTimes: []string{"x"}}
	normalizeSchedules(&cfg)
	if len(cfg.SendTimes) != 1 || len(cfg.TelegramSendTimes) != 1 {
		t.Fatalf("expected fallback default round, got %v / %v", cfg.SendTimes, cfg.TelegramSendTimes)
	}
}

func TestEffectiveTelegramToken(t *testing.T) {
	s := Settings{TelegramTokenEnv: "env-tok"}
	if s.EffectiveTelegramToken() != "env-tok" {
		t.Fatal("expected .env fallback")
	}
	s.TelegramToken = "web-tok"
	if s.EffectiveTelegramToken() != "web-tok" {
		t.Fatal("expected web value to take precedence")
	}
}

