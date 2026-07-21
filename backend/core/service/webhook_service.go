package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yi-nology/zentao-max/backend/core/models"
)

type WebhookService struct{}

func NewWebhookService() *WebhookService {
	return &WebhookService{}
}

type WebhookPayload struct {
	Title     string                  `json:"title"`
	Timestamp string                  `json:"timestamp"`
	Project   string                  `json:"project"`
	Summary   WebhookSummary          `json:"summary"`
	Details   []AssigneeDetailPayload `json:"details"`
	Message   string                  `json:"message"`
}

type WebhookSummary struct {
	Total           int            `json:"total"`
	HighSeverity    int            `json:"highSeverity"`
	StatusBreakdown map[string]int `json:"statusBreakdown"`
}

type AssigneeDetailPayload struct {
	Assignee     string `json:"assignee"`
	Account      string `json:"account"`
	Total        int    `json:"total"`
	HighSeverity int    `json:"highSeverity"`
	Fatal        int    `json:"fatal"`
	Serious      int    `json:"serious"`
	Moderate     int    `json:"moderate"`
	Minor        int    `json:"minor"`
}

func detectPlatform(url string) string {
	if strings.Contains(url, "apigw") && strings.Contains(url, "bot/hook/messages") {
		return "lanxin"
	}
	return "generic"
}

func genLanxinSign(secret string) (string, string) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	stringToSign := timestamp + "@" + secret
	h := hmac.New(sha256.New, []byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return signature, timestamp
}

func (s *WebhookService) buildPayload(report *models.BugReport) WebhookPayload {
	details := make([]AssigneeDetailPayload, 0, len(report.Details))
	for _, d := range report.Details {
		details = append(details, AssigneeDetailPayload{
			Assignee:     d.Assignee,
			Account:      d.Account,
			Total:        d.Total,
			HighSeverity: d.HighSeverity,
			Fatal:        d.Fatal,
			Serious:      d.Serious,
			Moderate:     d.Moderate,
			Minor:        d.Minor,
		})
	}
	return WebhookPayload{
		Title:     report.Title,
		Timestamp: report.Timestamp,
		Project:   report.ProjectName,
		Summary: WebhookSummary{
			Total:           report.Total,
			HighSeverity:    report.HighSeverity,
			StatusBreakdown: report.StatusBreakdown,
		},
		Details: details,
		Message: report.Message,
	}
}

func (s *WebhookService) buildLanxinBody(wh models.WebhookConfig, payload WebhookPayload) ([]byte, error) {
	msg := map[string]interface{}{
		"msgType": "text",
		"msgData": map[string]interface{}{
			"text": map[string]string{
				"content": payload.Message,
			},
		},
	}
	if wh.Secret != "" {
		sign, timestamp := genLanxinSign(wh.Secret)
		msg["sign"] = sign
		msg["timestamp"] = timestamp
	}
	return json.Marshal(msg)
}

func (s *WebhookService) SendAll(webhooks []models.WebhookConfig, report *models.BugReport) []models.WebhookResult {
	payload := s.buildPayload(report)
	results := make([]models.WebhookResult, 0, len(webhooks))

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, wh := range webhooks {
		if !wh.Enabled {
			continue
		}
		wg.Add(1)
		go func(wh models.WebhookConfig) {
			defer wg.Done()
			result := s.sendSingle(wh, payload)
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(wh)
	}
	wg.Wait()
	return results
}

func (s *WebhookService) sendSingle(wh models.WebhookConfig, payload WebhookPayload) models.WebhookResult {
	result := models.WebhookResult{
		WebhookID:   wh.ID,
		WebhookName: wh.Name,
		WebhookURL:  wh.URL,
	}

	platform := wh.Platform
	if platform == "" {
		platform = detectPlatform(wh.URL)
	}

	var body []byte
	var err error

	if platform == "lanxin" {
		body, err = s.buildLanxinBody(wh, payload)
	} else {
		body, err = json.Marshal(payload)
	}
	if err != nil {
		result.Error = err.Error()
		return result
	}

	client := &http.Client{Timeout: 15 * time.Second}
	if wh.SkipSSL {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	req, err := http.NewRequest("POST", wh.URL, bytes.NewReader(body))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	var respBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err == nil {
		if errCode, ok := respBody["errCode"]; ok {
			if code, ok := errCode.(float64); ok && code == 0 {
				result.Success = true
			} else {
				result.Success = false
				result.Error = fmt.Sprintf("errCode=%v errMsg=%v", respBody["errCode"], respBody["errMsg"])
			}
		} else {
			result.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
		}
	} else {
		result.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	}

	if !result.Success && result.Error == "" {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return result
}

func (s *WebhookService) TestWebhook(url string) (*models.WebhookResult, error) {
	platform := detectPlatform(url)
	testPayload := WebhookPayload{
		Title:     "Webhook 连通性测试",
		Timestamp: time.Now().Format(time.RFC3339),
		Project:   "测试项目",
		Summary: WebhookSummary{
			Total:           0,
			HighSeverity:    0,
			StatusBreakdown: map[string]int{"active": 0, "resolved": 0, "closed": 0},
		},
		Details: []AssigneeDetailPayload{},
		Message: "【提醒】这是一条测试消息，用于验证 Webhook 连通性。",
	}

	wh := models.WebhookConfig{URL: url, Platform: platform}
	result := s.sendSingle(wh, testPayload)
	return &result, nil
}

func (s *WebhookService) SendAllGeneric(webhooks []models.WebhookConfig, message string) []models.WebhookResult {
	payload := WebhookPayload{
		Title:     "",
		Timestamp: time.Now().Format(time.RFC3339),
		Project:   "",
		Summary:   WebhookSummary{},
		Details:   []AssigneeDetailPayload{},
		Message:   message,
	}
	results := make([]models.WebhookResult, 0, len(webhooks))

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, wh := range webhooks {
		if !wh.Enabled {
			continue
		}
		wg.Add(1)
		go func(wh models.WebhookConfig) {
			defer wg.Done()
			result := s.sendSingle(wh, payload)
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(wh)
	}
	wg.Wait()
	return results
}
