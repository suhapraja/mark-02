package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// apiVersion is the Graph API version used for all WhatsApp Cloud API
// calls. Meta sunsets versions roughly two years after release, so this
// is kept in one place to make future bumps a single-line change.
const apiVersion = "v21.0"

type Client struct {
	Token         string
	PhoneNumberID string
	httpClient    *http.Client
}

func NewClient(token, phoneNumberID string) *Client {
	return &Client{
		Token:         token,
		PhoneNumberID: phoneNumberID,
		httpClient:    &http.Client{},
	}
}

type sendTextRequest struct {
	MessagingProduct string      `json:"messaging_product"`
	To               string      `json:"to"`
	Type             string      `json:"type"`
	Text             sendText    `json:"text"`
}

type sendText struct {
	Body string `json:"body"`
}

// SendText sends a plain text WhatsApp message to the given phone number
// (format: country code + number, no "+", e.g. "62812xxxxxxx").
func (c *Client) SendText(to, body string) error {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", apiVersion, c.PhoneNumberID)

	reqBody := sendTextRequest{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "text",
		Text:             sendText{Body: body},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SendTemplate sends an approved message template. Unlike SendText this
// is delivered even outside WhatsApp's 24-hour customer service window,
// which is the only way to reach someone who hasn't messaged recently.
//
// Body parameters must be non-empty and single-line: WhatsApp rejects
// newlines, tabs, and runs of 4+ spaces inside a parameter, so callers
// should pass values through SanitizeParam.
func (c *Client) SendTemplate(to, name, langCode string, bodyParams []string) error {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", apiVersion, c.PhoneNumberID)

	params := make([]map[string]string, 0, len(bodyParams))
	for _, p := range bodyParams {
		params = append(params, map[string]string{"type": "text", "text": SanitizeParam(p)})
	}

	template := map[string]any{
		"name":     name,
		"language": map[string]string{"code": langCode},
	}
	if len(params) > 0 {
		template["components"] = []map[string]any{
			{"type": "body", "parameters": params},
		}
	}

	reqBody := map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "template",
		"template":          template,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp template error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SanitizeParam makes a value safe to use as a template parameter:
// collapses all whitespace (newlines included) to single spaces and
// substitutes a placeholder for empty values, which WhatsApp rejects.
func SanitizeParam(s string) string {
	collapsed := strings.Join(strings.Fields(s), " ")
	if collapsed == "" {
		return "-"
	}
	return collapsed
}

// UploadMedia uploads a file to WhatsApp's media endpoint and returns a
// media ID, which can then be passed to SendDocument.
func (c *Client) UploadMedia(filename string, data []byte, mimeType string) (string, error) {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/media", apiVersion, c.PhoneNumberID)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("messaging_product", "whatsapp"); err != nil {
		return "", err
	}
	if err := writer.WriteField("type", mimeType); err != nil {
		return "", err
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload media: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("whatsapp media upload error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse upload response: %w", err)
	}

	return result.ID, nil
}

// SendDocument sends a file (e.g. the Excel export) to the given phone
// number. WhatsApp requires the file to be uploaded first to get a media
// ID, then referenced in the message — see Meta's media upload docs.
// This is a placeholder to fill in once media upload is wired up; for now
// the Excel export is generated and can be sent via a document message
// using the returned media ID from UploadMedia (implement alongside).
func (c *Client) SendDocument(to, mediaID, filename string) error {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", apiVersion, c.PhoneNumberID)

	reqBody := map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "document",
		"document": map[string]string{
			"id":       mediaID,
			"filename": filename,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
