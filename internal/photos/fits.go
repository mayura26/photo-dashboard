package photos

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Frame ids a photo can be cropped into. A photo may fit more than one.
const (
	FrameCover  = "cover"
	FramePair   = "pair"
	FrameSquare = "square"
	FrameWide   = "wide"
)

var frameOrder = []string{FrameCover, FramePair, FrameSquare, FrameWide}

var fitsHTTP = &http.Client{Timeout: 60 * time.Second}

// ParseFits reads a model reply. It accepts a bare JSON object or a fenced block.
// Unknown ids are dropped. ok is false when nothing usable remains.
func ParseFits(raw string) ([]string, bool) {
	body := extractJSONObject(raw)
	if body == "" {
		return nil, false
	}
	var loose struct {
		Fits json.RawMessage `json:"fits"`
	}
	if err := json.Unmarshal([]byte(body), &loose); err != nil || len(bytes.TrimSpace(loose.Fits)) == 0 {
		return nil, false
	}
	var ids []string
	if err := json.Unmarshal(loose.Fits, &ids); err != nil {
		return nil, false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if !knownFrame(id) || seen[id] {
			continue
		}
		seen[id] = true
	}
	if len(seen) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(seen))
	for _, id := range frameOrder {
		if seen[id] {
			out = append(out, id)
		}
	}
	return out, true
}

// ClampFits drops frames the photo's orientation cannot survive.
// Portrait keeps pair and square. Landscape keeps cover, wide, and square.
// An empty result becomes the orientation fallback so a bad reply is not retried.
func ClampFits(fits []string, orient string) []string {
	allow := allowedFrames(orient)
	if allow == nil {
		return copyKnown(fits)
	}
	out := make([]string, 0, len(allow))
	seen := map[string]bool{}
	for _, id := range fits {
		id = strings.ToLower(strings.TrimSpace(id))
		seen[id] = true
	}
	for _, id := range allow {
		if seen[id] {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), fallbackFrames(orient)...)
	}
	return out
}

func allowedFrames(orient string) []string {
	switch orient {
	case "p":
		return []string{FramePair, FrameSquare}
	case "l":
		return []string{FrameCover, FrameSquare, FrameWide}
	default:
		return nil
	}
}

func fallbackFrames(orient string) []string {
	switch orient {
	case "p":
		return []string{FramePair, FrameSquare}
	case "l":
		return []string{FrameCover, FrameWide}
	default:
		return nil
	}
}

func copyKnown(fits []string) []string {
	seen := map[string]bool{}
	for _, id := range fits {
		id = strings.ToLower(strings.TrimSpace(id))
		if knownFrame(id) {
			seen[id] = true
		}
	}
	out := make([]string, 0, len(seen))
	for _, id := range frameOrder {
		if seen[id] {
			out = append(out, id)
		}
	}
	return out
}

// ClassifyFits asks GPT-5.4 which frames this display JPEG survives.
// ok is false when the key is missing or the reply cannot be used.
func ClassifyFits(jpeg []byte) ([]string, bool) {
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" || len(jpeg) == 0 {
		return nil, false
	}
	raw, err := completeFits(key, jpeg)
	if err != nil {
		log.Printf("photos: frame fit: %v", err)
		return nil, false
	}
	fits, ok := ParseFits(raw)
	if !ok {
		log.Printf("photos: frame fit unreadable: %s", truncate(raw, 180))
		return nil, false
	}
	return fits, true
}

func completeFits(key string, jpeg []byte) (string, error) {
	payload := map[string]any{
		"model": openAIModel,
		"messages": []map[string]any{
			{
				"role": "system",
				"content": strings.Join([]string{
					"You decide which photo-frame crops keep this image's subject intact.",
					"The frame uses object-fit cover, anchored at 50% across and 30% down, so the bottom is cropped before the top.",
					"Reply with JSON only, no markdown.",
					`{"fits":["pair","square"]}`,
					"Include only the frames where the subject stays readable. Do not list every frame.",
					"cover: full stage 16:10 (1280x800). Horizontal scenes only. Never use cover for a vertical photo, or when the subject is a standing person.",
					"pair: tall column about 4:5 (637x800). Standing people and other tall subjects, shown side by side.",
					"square: near 1:1 (764x794), the left slot of a three-photo frame. Squares, mild portraits, and landscapes with a central subject.",
					"wide: horizontal about 5:4 (510x397), the right slots of that frame. Horizontal scenes only. Omit for a vertical photo and when a person would lose their head or feet.",
					"A vertical photo of a person is pair and maybe square. It must not include cover or wide.",
				}, " "),
			},
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "Which frames does this photo survive?"},
					{"type": "image_url", "image_url": map[string]string{
						"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpeg),
					}},
				},
			},
		},
		"max_completion_tokens": 80,
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
	res, err := fitsHTTP.Do(req)
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

func knownFrame(id string) bool {
	switch id {
	case FrameCover, FramePair, FrameSquare, FrameWide:
		return true
	default:
		return false
	}
}

func extractJSONObject(s string) string {
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

func photoOrient(p map[string]any) string {
	o, _ := p["o"].(string)
	return o
}

func photoFitsList(p map[string]any) []string {
	switch v := p["fits"].(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func sameFits(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func photoHasFits(p map[string]any) bool {
	raw, ok := p["fits"]
	if !ok || raw == nil {
		return false
	}
	switch v := raw.(type) {
	case []string:
		return len(v) > 0
	case []any:
		return len(v) > 0
	default:
		return false
	}
}
