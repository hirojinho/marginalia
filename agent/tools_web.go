package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

// webFetchLimiter is a per-process simple rate limiter on web_fetch
// calls. It's a singleton because the rate limit is a property of the
// outbound HTTP behavior, not of any one App instance.
var (
	webFetchMu    sync.Mutex
	webFetchTimes []time.Time
)

// ToolWebFetch fetches a URL and returns its content as plain text or
// converted Markdown. It enforces a rolling 5-per-minute rate limit and
// accepts only http:// and https:// URLs.
func ToolWebFetch(args json.RawMessage) string {
	var p struct{ URL string }
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if !strings.HasPrefix(p.URL, "http://") && !strings.HasPrefix(p.URL, "https://") {
		return "error: only http:// and https:// URLs are allowed"
	}

	if wait := reserveWebFetchSlot(); wait > 0 {
		return fmt.Sprintf("rate limited: try again in %s", wait)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", p.URL, nil)
	if err != nil {
		return "error: " + err.Error()
	}
	req.Header.Set("User-Agent", "StudyAgent/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "error fetching URL: " + err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Sprintf("error: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500000))
	if err != nil {
		return "error reading response: " + err.Error()
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		return webBodyToMarkdown(p.URL, body)
	}

	text := string(body)
	if len(text) > 50000 {
		text = text[:50000] + "\n\n[...truncated at 50,000 characters]"
	}
	return fmt.Sprintf("Source: %s\n\n%s", p.URL, text)
}

// webBodyToMarkdown converts an HTML body to Markdown and formats it with a
// source/title header. Called by ToolWebFetch when the response is text/html.
func webBodyToMarkdown(url string, body []byte) string {
	converter := md.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(string(body))
	if err != nil {
		return "error converting HTML: " + err.Error()
	}
	if len(markdown) > 50000 {
		markdown = markdown[:50000] + "\n\n[...truncated at 50,000 characters]"
	}
	title := ""
	if idx := strings.Index(markdown, "# "); idx != -1 {
		end := strings.Index(markdown[idx:], "\n")
		if end != -1 {
			title = markdown[idx+2 : idx+end]
		}
	}
	result := fmt.Sprintf("Source: %s", url)
	if title != "" {
		result += "\nTitle: " + title
	}
	result += "\n\n" + markdown
	return result
}

// reserveWebFetchSlot enforces a rolling 5-per-minute limit. Returns
// the duration to wait before retrying, or 0 if a slot was reserved.
func reserveWebFetchSlot() time.Duration {
	webFetchMu.Lock()
	defer webFetchMu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)

	recent := webFetchTimes[:0]
	for _, t := range webFetchTimes {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	webFetchTimes = recent

	if len(recent) >= 5 {
		return recent[0].Add(time.Minute).Sub(now).Round(time.Second)
	}
	webFetchTimes = append(webFetchTimes, now)
	return 0
}
