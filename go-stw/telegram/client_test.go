package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientGetMe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/getMe") {
			http.NotFound(w, r)
			return
		}
		resp := apiResponse{
			OK: true,
			Result: json.RawMessage(`{"id":123456,"is_bot":true,"first_name":"STWBot","username":"stw_bot"}`),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient("dummy_token")
	client.SetBaseURL(ts.URL)

	user, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Username != "stw_bot" || user.ID != 123456 {
		t.Errorf("unexpected user: %+v", user)
	}
}

func TestClientSendMessageSplit(t *testing.T) {
	var sent []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		sent = append(sent, req["text"].(string))
		resp := apiResponse{
			OK: true,
			Result: json.RawMessage(`{"message_id":1}`),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient("dummy_token")
	client.SetBaseURL(ts.URL)

	longText := strings.Repeat("A\n", 3000) // 6000 bytes
	err := client.SendMessage(context.Background(), 1234, longText)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if len(sent) < 2 {
		t.Errorf("expected text to be split into chunks, got %d chunks", len(sent))
	}
}

func TestSplitMessage(t *testing.T) {
	txt := "Line 1\nLine 2\nLine 3"
	chunks := SplitMessage(txt, 10)
	if len(chunks) < 2 {
		t.Errorf("expected split, got %v", chunks)
	}
}

