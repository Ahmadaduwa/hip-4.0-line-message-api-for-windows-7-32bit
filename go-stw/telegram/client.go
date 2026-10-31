package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.telegram.org"
	maxMessageLen  = 4000 // ขีดจำกัด Telegram คือ 4096 ตัวอักษร ตัดที่ 4000 เพื่อความปลอดภัย
)

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"` // "private", "group", "supergroup", "channel"
	Title    string `json:"title,omitempty"`
	Username string `json:"username,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      Chat   `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text,omitempty"`
}

type ChatMemberUpdated struct {
	Chat          Chat `json:"chat"`
	From          User `json:"from"`
	Date          int64 `json:"date"`
	OldChatMember struct {
		Status string `json:"status"` // "member", "left", "kicked", etc.
		User   User   `json:"user"`
	} `json:"old_chat_member"`
	NewChatMember struct {
		Status string `json:"status"`
		User   User   `json:"user"`
	} `json:"new_chat_member"`
}

type Update struct {
	UpdateID          int64               `json:"update_id"`
	Message           *Message            `json:"message,omitempty"`
	MyChatMember      *ChatMemberUpdated  `json:"my_chat_member,omitempty"`
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Parameters  *struct {
		RetryAfter int `json:"retry_after,omitempty"`
	} `json:"parameters,omitempty"`
}

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token:   token,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 35 * time.Second,
		},
	}
}

func (c *Client) SetBaseURL(url string) {
	c.baseURL = strings.TrimRight(url, "/")
}

func (c *Client) apiURL(method string) string {
	return fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.token, method)
}

func (c *Client) post(ctx context.Context, method string, payload interface{}, out interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL(method), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var apiRes apiResponse
	if err := json.Unmarshal(respBytes, &apiRes); err != nil {
		return fmt.Errorf("decode response error: %w (raw: %s)", err, string(respBytes))
	}

	if !apiRes.OK {
		if apiRes.Parameters != nil && apiRes.Parameters.RetryAfter > 0 {
			return fmt.Errorf("rate limited: retry after %d seconds", apiRes.Parameters.RetryAfter)
		}
		return fmt.Errorf("telegram api error (%d): %s", apiRes.ErrorCode, apiRes.Description)
	}

	if out != nil && len(apiRes.Result) > 0 {
		return json.Unmarshal(apiRes.Result, out)
	}
	return nil
}

func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var user User
	err := c.post(ctx, "getMe", map[string]interface{}{}, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) SendMessage(ctx context.Context, chatID interface{}, text string) error {
	chunks := SplitMessage(text, maxMessageLen)
	for _, chunk := range chunks {
		payload := map[string]interface{}{
			"chat_id": chatID,
			"text":    chunk,
		}
		var msg Message
		err := c.post(ctx, "sendMessage", payload, &msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	payload := map[string]interface{}{
		"offset":  offset,
		"timeout": timeoutSec,
		"allowed_updates": []string{"message", "my_chat_member"},
	}
	var updates []Update
	err := c.post(ctx, "getUpdates", payload, &updates)
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func SplitMessage(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	lines := strings.Split(text, "\n")
	var current strings.Builder

	for _, line := range lines {
		if current.Len()+len(line)+1 > limit && current.Len() > 0 {
			chunks = append(chunks, strings.TrimRight(current.String(), "\n"))
			current.Reset()
		}

		if len(line) > limit {
			// ถ้าบรรทัดเดียวเกิน limit ให้ตัดแบ่งเป็นท่อนๆ
			runes := []rune(line)
			for len(runes) > limit {
				chunks = append(chunks, string(runes[:limit]))
				runes = runes[limit:]
			}
			current.WriteString(string(runes))
			current.WriteString("\n")
		} else {
			current.WriteString(line)
			current.WriteString("\n")
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimRight(current.String(), "\n"))
	}
	return chunks
}

