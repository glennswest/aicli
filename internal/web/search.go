package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

type WebSearch struct {
	client *http.Client
}

func NewSearch() *WebSearch {
	return &WebSearch{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (w *WebSearch) Search(query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	// Use DuckDuckGo HTML version (no API key needed)
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseResults(string(body), maxResults), nil
}

func parseResults(html string, max int) []SearchResult {
	var results []SearchResult

	// Extract result blocks
	resultRegex := regexp.MustCompile(`<a rel="nofollow" class="result__a" href="([^"]+)"[^>]*>([^<]+)</a>`)
	snippetRegex := regexp.MustCompile(`<a class="result__snippet"[^>]*>([^<]+)</a>`)

	matches := resultRegex.FindAllStringSubmatch(html, max)
	snippets := snippetRegex.FindAllStringSubmatch(html, max)

	for i, match := range matches {
		if len(match) < 3 {
			continue
		}

		result := SearchResult{
			URL:   decodeURL(match[1]),
			Title: cleanHTML(match[2]),
		}

		if i < len(snippets) && len(snippets[i]) > 1 {
			result.Snippet = cleanHTML(snippets[i][1])
		}

		if result.URL != "" && result.Title != "" {
			results = append(results, result)
		}
	}

	return results
}

func decodeURL(encoded string) string {
	// DuckDuckGo encodes URLs, extract the actual URL
	if strings.Contains(encoded, "uddg=") {
		parts := strings.Split(encoded, "uddg=")
		if len(parts) > 1 {
			decoded, err := url.QueryUnescape(parts[1])
			if err == nil {
				// Remove any trailing parameters
				if idx := strings.Index(decoded, "&"); idx > 0 {
					decoded = decoded[:idx]
				}
				return decoded
			}
		}
	}
	return encoded
}

func cleanHTML(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.TrimSpace(s)
	return s
}

func (w *WebSearch) FetchPage(pageURL string) (string, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Basic HTML to text conversion
	return htmlToText(string(body)), nil
}

func htmlToText(html string) string {
	// Remove script and style tags
	scriptRegex := regexp.MustCompile(`<script[^>]*>[\s\S]*?</script>`)
	styleRegex := regexp.MustCompile(`<style[^>]*>[\s\S]*?</style>`)
	html = scriptRegex.ReplaceAllString(html, "")
	html = styleRegex.ReplaceAllString(html, "")

	// Replace common tags with newlines
	html = regexp.MustCompile(`<br\s*/?\s*>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`</(p|div|h[1-6]|li|tr)>`).ReplaceAllString(html, "\n")

	// Remove all remaining tags
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	text := tagRegex.ReplaceAllString(html, "")

	// Clean up whitespace
	text = cleanHTML(text)
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	result := strings.Join(cleaned, "\n")

	// Truncate if too long
	if len(result) > 8000 {
		result = result[:8000] + "\n... (truncated)"
	}

	return result
}
