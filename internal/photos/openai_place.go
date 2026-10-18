package photos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	openAIURL   = "https://api.openai.com/v1/chat/completions"
	openAIModel = "gpt-5.4"
)

var openAIHTTP = &http.Client{Timeout: 45 * time.Second}

// synthesizePlace asks GPT-5.4 for a short place caption from EXIF/XMP tags
// when GPS reverse-geocode produced nothing. Returns "" if disabled or unsure.
func synthesizePlace(h PlaceHints) string {
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" || !h.useful() {
		return ""
	}

	block := h.promptBlock()
	if strings.TrimSpace(block) == "" {
		return ""
	}

	payload := map[string]any{
		"model": openAIModel,
		"messages": []map[string]string{
			{
				"role": "system",
				"content": strings.Join([]string{
					"You label family photo-frame captions.",
					"Given filename and photo metadata/tags, reply with ONE short place or moment label suitable for a caption.",
					"Prefer real place names when tags suggest them (city, landmark, neighbourhood, venue).",
					"Otherwise a short evocative label is ok (e.g. \"Backyard\", \"Beach day\", \"Cottage weekend\").",
					"Max 40 characters. No quotes, no trailing punctuation, no explanation.",
					"If the metadata is useless camera junk with no place/moment clue, reply exactly: NONE",
				}, " "),
			},
			{
				"role":    "user",
				"content": "Synthesize a place/caption label from:\n" + block,
			},
		},
		"max_completion_tokens": 40,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ""
	}

	req, err := http.NewRequest(http.MethodPost, openAIURL, bytes.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	res, err := openAIHTTP.Do(req)
	if err != nil {
		log.Printf("photos: openai place: %v", err)
		return ""
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		log.Printf("photos: openai place status %d: %s", res.StatusCode, truncate(string(raw), 200))
		return ""
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return ""
	}
	label := strings.TrimSpace(out.Choices[0].Message.Content)
	label = strings.Trim(label, `"'`)
	label = strings.TrimRight(label, " .")
	if label == "" || strings.EqualFold(label, "NONE") || strings.EqualFold(label, "unknown") {
		return ""
	}
	if len(label) > 60 {
		label = strings.TrimSpace(label[:60])
	}
	return label
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("…(%d)", len(s))
}
