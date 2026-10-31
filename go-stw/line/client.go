package line

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"stw_teacher_check/logger"
)

type Client struct {
	Token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // รองรับ Windows 7 ที่ไม่ได้อัปเดต Root Certificate Store
		},
	}
	return &Client{
		Token: strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: tr,
		},
	}
}

func (c *Client) BroadcastText(text string) error {
	if c.Token == "" {
		return fmt.Errorf("ไม่มี LINE Channel Access Token ในระบบ")
	}

	payload := map[string]interface{}{
		"messages": []map[string]string{
			{
				"type": "text",
				"text": text,
			},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.line.me/v2/bot/message/broadcast", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("LINE", err, "ส่ง LINE Broadcast ล้มเหลว")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Info("LINE", "ส่งข้อความ Broadcast ไปยังเพื่อนทุกคนของบอทสำเร็จ")
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	err = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	logger.Error("LINE", err, "ส่ง LINE Broadcast ล้มเหลว (HTTP %d)", resp.StatusCode)
	return err
}

func (c *Client) SendText(targets []string, text string) error {
	if c.Token == "" {
		return fmt.Errorf("ไม่มี LINE Channel Access Token ในระบบ")
	}
	if len(targets) == 0 {
		return c.BroadcastText(text)
	}

	var firstErr error
	successCount := 0

	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}

		payload := map[string]interface{}{
			"to": target,
			"messages": []map[string]string{
				{
					"type": "text",
					"text": text,
				},
			},
		}

		b, err := json.Marshal(payload)
		if err != nil {
			continue
		}

		req, err := http.NewRequest("POST", "https://api.line.me/v2/bot/message/push", bytes.NewReader(b))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.Token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			logger.Error("LINE", err, "ส่ง LINE ไปยัง %s ล้มเหลว", target)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			successCount++
		} else {
			err := fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			logger.Error("LINE", err, "ส่ง LINE ไปยัง %s ล้มเหลว (HTTP %d)", target, resp.StatusCode)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	logger.Info("LINE", "ส่งข้อความไปยังผู้รับสำเร็จ %d/%d รายการ", successCount, len(targets))
	if successCount == 0 && firstErr != nil {
		return firstErr
	}
	return nil
}

func (c *Client) ReplyText(replyToken, text string) error {
	if c.Token == "" || replyToken == "" {
		return nil
	}
	payload := map[string]interface{}{
		"replyToken": replyToken,
		"messages": []map[string]string{
			{
				"type": "text",
				"text": text,
			},
		},
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.line.me/v2/bot/message/reply", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) GetProfile(userID string) (name, pictureURL string) {
	if c.Token == "" || userID == "" {
		return "", ""
	}
	url := fmt.Sprintf("https://api.line.me/v2/bot/profile/%s", userID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return "", ""
	}
	defer resp.Body.Close()

	var data struct {
		DisplayName string `json:"displayName"`
		PictureURL  string `json:"pictureUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
		return data.DisplayName, data.PictureURL
	}
	return "", ""
}

func (c *Client) GetGroupSummary(groupID string) (name, pictureURL string) {
	if c.Token == "" || groupID == "" {
		return "", ""
	}
	url := fmt.Sprintf("https://api.line.me/v2/bot/group/%s/summary", groupID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return "", ""
	}
	defer resp.Body.Close()

	var data struct {
		GroupName  string `json:"groupName"`
		PictureURL string `json:"pictureUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
		return data.GroupName, data.PictureURL
	}
	return "", ""
}

type BotInfo struct {
	UserID      string `json:"userId"`
	BasicID     string `json:"basicId"`
	DisplayName string `json:"displayName"`
	PictureURL  string `json:"pictureUrl"`
	ChatMode    string `json:"chatMode"`
}

func (c *Client) GetBotInfo() (*BotInfo, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("ยังไม่ได้ระบุ Channel Access Token")
	}
	req, err := http.NewRequest("GET", "https://api.line.me/v2/bot/info", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ไม่สามารถติดต่อ LINE API ได้: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("LINE API ปฏิเสธ: %s (HTTP %d)", errResp.Message, resp.StatusCode)
		}
		return nil, fmt.Errorf("LINE API ปฏิเสธ (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var info BotInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *Client) GetWebhookEndpoint() (string, bool, error) {
	if c.Token == "" {
		return "", false, fmt.Errorf("ยังไม่ได้ระบุ Channel Access Token")
	}
	req, err := http.NewRequest("GET", "https://api.line.me/v2/bot/channel/webhook/endpoint", nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var data struct {
		Endpoint string `json:"endpoint"`
		Active   bool   `json:"active"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", false, err
	}
	return data.Endpoint, data.Active, nil
}

func (c *Client) SetWebhookEndpoint(webhookURL string) error {
	if c.Token == "" {
		return fmt.Errorf("ยังไม่ได้ระบุ Channel Access Token")
	}
	webhookURL = strings.TrimSpace(webhookURL)
	if !strings.HasPrefix(webhookURL, "https://") {
		return fmt.Errorf("Webhook URL ต้องขึ้นต้นด้วย https:// เท่านั้น")
	}

	payload := map[string]string{
		"endpoint": webhookURL,
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequest("PUT", "https://api.line.me/v2/bot/channel/webhook/endpoint", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Info("LINE", "✅ ตั้งค่า Webhook URL ใน LINE Developers อัตโนมัติสำเร็จ: %s", webhookURL)
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("ตั้งค่า Webhook ไม่สำเร็จ (HTTP %d): %s", resp.StatusCode, string(body))
}

func ValidateSignature(channelSecret string, body []byte, signature string) bool {
	if channelSecret == "" || signature == "" {
		return true // skip if secret is not set
	}
	mac := hmac.New(sha256.New, []byte(channelSecret))
	mac.Write(body)
	expectedMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedMAC), []byte(signature))
}
