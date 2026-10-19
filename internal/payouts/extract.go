package payouts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	openAIURL   = "https://api.openai.com/v1/chat/completions"
	openAIModel = "gpt-5.4"
)

var (
	openAIHTTP  = &http.Client{Timeout: 60 * time.Second}
	dateRe      = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	dateLayouts = []string{
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		"02/01/2006",
		"2 Jan 2006",
		"2 January 2006",
		"Jan 2, 2006",
		"January 2, 2006",
		"Jan 2 2006",
		"January 2 2006",
	}
)

// Extracted is the payout fields read from a screenshot.
type Extracted struct {
	Amount   *float64
	Currency string
	Date     string
	Account  string
	FirmID   string
	OK       bool
}

// Extract asks GPT-5.4 to read a payout screenshot. OK is false when the
// key is missing or the model reply cannot be parsed. Callers still keep the image.
func Extract(jpeg []byte, firms []Firm) Extracted {
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" || len(jpeg) == 0 {
		return Extracted{}
	}
	raw, err := completeVision(key, jpeg, firms)
	if err != nil {
		log.Printf("payouts: openai: %v", err)
		return Extracted{}
	}
	out, ok := ParseExtract(raw)
	if !ok {
		log.Printf("payouts: openai unreadable: %s", truncate(raw, 180))
		return Extracted{}
	}
	return out
}

// ParseExtract reads a model reply. It accepts a bare JSON object or a fenced block.
func ParseExtract(raw string) (Extracted, bool) {
	body := extractJSON(raw)
	if body == "" {
		return Extracted{}, false
	}
	var loose struct {
		Amount   json.RawMessage `json:"amount"`
		Currency string          `json:"currency"`
		Date     string          `json:"date"`
		Account  string          `json:"account"`
		FirmID   string          `json:"firmId"`
	}
	if err := json.Unmarshal([]byte(body), &loose); err != nil {
		return Extracted{}, false
	}
	amount, ok := parseAmount(loose.Amount)
	if !ok {
		return Extracted{}, false
	}
	account := strings.Trim(strings.TrimSpace(loose.Account), `"'`)
	if len(account) > 80 {
		account = account[:80]
	}
	return Extracted{
		Amount:   amount,
		Currency: cleanCurrency(loose.Currency),
		Date:     cleanDate(loose.Date),
		Account:  account,
		FirmID:   strings.TrimSpace(loose.FirmID),
		OK:       true,
	}, true
}

func completeVision(key string, jpeg []byte, firms []Firm) (string, error) {
	prompt := "Read this prop-trading payout screenshot.\n" + firmBlock(firms)
	payload := map[string]any{
		"model": openAIModel,
		"messages": []map[string]any{
			{
				"role": "system",
				"content": strings.Join([]string{
					"You extract one prop-firm payout from a screenshot.",
					"Reply with JSON only, no markdown.",
					`{"amount": number or null, "currency": "USD", "date": "YYYY-MM-DD" or "", "account": "", "firmId": ""}`,
					"amount is money paid out to the trader, not the account balance, profit target, or drawdown.",
					"date is the payout date when one is visible, otherwise an empty string.",
					"account is the prop account name or id shown on the screenshot.",
					"firmId must be one of the provided firm ids, or empty if you are not sure.",
					"Use null for amount when the screenshot does not show a payout amount.",
				}, " "),
			},
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": prompt},
					{"type": "image_url", "image_url": map[string]string{
						"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpeg),
					}},
				},
			},
		},
		"max_completion_tokens": 300,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, openAIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	res, err := openAIHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("status %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("no choices")
	}
	return out.Choices[0].Message.Content, nil
}

func firmBlock(firms []Firm) string {
	if len(firms) == 0 {
		return "No firms are configured. Leave firmId empty."
	}
	var b strings.Builder
	b.WriteString("Firms:\n")
	for _, f := range firms {
		b.WriteString("- id=")
		b.WriteString(f.ID)
		b.WriteString(" name=")
		b.WriteString(f.Name)
		if len(f.Prefixes) > 0 {
			b.WriteString(" prefixes=")
			b.WriteString(strings.Join(f.Prefixes, ","))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j < i {
		return ""
	}
	return s[i : j+1]
}

func parseAmount(raw json.RawMessage) (*float64, bool) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, true
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return &n, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false
	}
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "null") || strings.EqualFold(s, "none") {
		return nil, true
	}
	var b strings.Builder
	dot := false
	for _, r := range s {
		if r == '-' && b.Len() == 0 {
			b.WriteRune(r)
			continue
		}
		if r == '.' && !dot {
			dot = true
			b.WriteRune(r)
			continue
		}
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 || b.String() == "-" || b.String() == "." || b.String() == "-." {
		return nil, true
	}
	f, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		return nil, false
	}
	return &f, true
}

func cleanCurrency(s string) string {
	s = strings.TrimSpace(s)
	switch s {
	case "$", "US$", "USD$":
		return "USD"
	case "€", "EUR€":
		return "EUR"
	case "£":
		return "GBP"
	}
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	if len(out) > 3 {
		return out[:3]
	}
	if len(out) < 3 {
		return ""
	}
	return out
}

func cleanDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "null") || strings.EqualFold(s, "none") {
		return ""
	}
	if m := dateRe.FindString(s); m != "" {
		if _, err := time.Parse("2006-01-02", m); err == nil {
			return m
		}
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
