package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stw_teacher_check/config"
)

func TestMaskSecret(t *testing.T) {
	if maskSecret("") != "" {
		t.Errorf("expected empty string")
	}
	if maskSecret("12345") != "******" {
		t.Errorf("expected ****** for short string")
	}
	if maskSecret("1234567890abcdef") != "1234...cdef" {
		t.Errorf("unexpected masked: %s", maskSecret("1234567890abcdef"))
	}
}

func TestWebAuthSession(t *testing.T) {
	ws := NewWebServer(nil, nil, nil, nil)
	token := ws.createSession()
	if !ws.isValidSession(token) {
		t.Fatalf("session should be valid")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		tok := ws.getAuthToken(r)
		ws.jsonResponse(rw, http.StatusOK, map[string]bool{"valid": ws.isValidSession(tok)})
	})
	handler.ServeHTTP(w, req)

	var res map[string]bool
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if !res["valid"] {
		t.Errorf("auth token extraction failed")
	}
}

func TestSaveSettingsValidation(t *testing.T) {
	// ตรวจสอบว่า NormalizeTimes ถูกเรียกและทำงานถูกต้อง
	cfg := config.Settings{
		SendTimes: []string{"09:05", "18:00"},
		TelegramSendTimes: []string{"09:05", "12:00"},
	}
	saved, err := config.SaveSettings(cfg)
	if err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}
	if len(saved.SendTimes) != 2 || len(saved.TelegramSendTimes) != 2 {
		t.Fatalf("times not saved correctly: %+v", saved)
	}
}

