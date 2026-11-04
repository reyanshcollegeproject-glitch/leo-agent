package search

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type Engine struct {
	client *http.Client
}

func NewEngine() *Engine {
	return &Engine{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Search performs a real-time web search and returns ranked results
func (e *Engine) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	form := url.Values{}
	form.Set("q", query)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://lite.duckduckgo.com/lite/", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	htmlContent := string(bodyBytes)
	results := parseResults(htmlContent, maxResults)

	if len(results) == 0 {
		// Fallback to Wikipedia Instant Search if DuckDuckGo returned no snippets
		return e.searchWikipedia(ctx, query, maxResults)
	}

	return results, nil
}

func parseResults(htmlContent string, maxResults int) []SearchResult {
	var results []SearchResult

	// Match any <a> tag with class containing result-link
	linkRegex := regexp.MustCompile(`(?i)<a\s+[^>]*class=['"][^'"]*result-link[^'"]*['"][^>]*>([\s\S]*?)</a>`)
	hrefRegex := regexp.MustCompile(`(?i)href=['"]([^'"]+)['"]`)
	snippetRegex := regexp.MustCompile(`(?i)<td\s+[^>]*class=['"][^'"]*result-snippet[^'"]*['"][^>]*>([\s\S]*?)</td>`)
	stripTags := regexp.MustCompile(`<[^>]*>`)

	linkMatches := linkRegex.FindAllStringSubmatch(htmlContent, -1)
	snippetMatches := snippetRegex.FindAllStringSubmatch(htmlContent, -1)

	count := len(linkMatches)
	if count > maxResults {
		count = maxResults
	}

	for i := 0; i < count; i++ {
		fullLinkTag := linkMatches[i][0]
		rawTitle := linkMatches[i][1]

		hrefSub := hrefRegex.FindStringSubmatch(fullLinkTag)
		if len(hrefSub) < 2 {
			continue
		}
		rawURL := hrefSub[1]

		rawSnippet := ""
		if i < len(snippetMatches) {
			rawSnippet = snippetMatches[i][1]
		}

		actualURL := extractActualURL(rawURL)
		cleanTitle := cleanText(stripTags.ReplaceAllString(rawTitle, ""))
		cleanSnippet := cleanText(stripTags.ReplaceAllString(rawSnippet, ""))

		if cleanTitle != "" && actualURL != "" {
			results = append(results, SearchResult{
				Title:   cleanTitle,
				URL:     actualURL,
				Snippet: cleanSnippet,
			})
		}
	}

	return results
}

func extractActualURL(rawURL string) string {
	if strings.Contains(rawURL, "uddg=") {
		u, err := url.Parse(rawURL)
		if err == nil {
			if target := u.Query().Get("uddg"); target != "" {
				return target
			}
		}
	}
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	return rawURL
}

func cleanText(s string) string {
	decoded := html.UnescapeString(s)
	// normalize whitespace
	fields := strings.Fields(decoded)
	return strings.Join(fields, " ")
}

// Fallback search using Wikipedia OpenSearch API
func (e *Engine) searchWikipedia(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	apiURL := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=opensearch&search=%s&limit=%d&namespace=0&format=json",
		url.QueryEscape(query), maxResults)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "MacPilotAI/1.0 (Autonomous Agent)")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || len(data) < 4 {
		return nil, fmt.Errorf("no search results found")
	}

	titles, _ := data[1].([]interface{})
	snippets, _ := data[2].([]interface{})
	urls, _ := data[3].([]interface{})

	var results []SearchResult
	for i := 0; i < len(titles) && i < maxResults; i++ {
		title, _ := titles[i].(string)
		snippet, _ := snippets[i].(string)
		targetURL, _ := urls[i].(string)
		if title != "" {
			results = append(results, SearchResult{
				Title:   title,
				URL:     targetURL,
				Snippet: snippet,
			})
		}
	}

	return results, nil
}



// FetchPageText extracts readable text content from a web page
func (e *Engine) FetchPageText(ctx context.Context, targetURL string, maxChars int) (string, error) {
	if maxChars <= 0 {
		maxChars = 2000
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", err
	}

	raw := string(bodyBytes)

	// Remove <script> and <style> blocks
	stripScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	stripStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	stripTags := regexp.MustCompile(`<[^>]*>`)

	cleaned := stripScript.ReplaceAllString(raw, "")
	cleaned = stripStyle.ReplaceAllString(cleaned, "")
	cleaned = stripTags.ReplaceAllString(cleaned, " ")
	text := cleanText(cleaned)

	if len(text) > maxChars {
		text = text[:maxChars] + "..."
	}

	return text, nil
}

// FormatForLLM produces clean markdown citations for Meta AI Muse
func FormatForLLM(results []SearchResult) string {
	if len(results) == 0 {
		return "No web search results found for query."
	}
	var sb strings.Builder
	sb.WriteString("### Live Web Search Results:\n")
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, r.Title))
		if r.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", r.Snippet))
		}
		sb.WriteString(fmt.Sprintf("   *Source*: %s\n\n", r.URL))
	}
	return sb.String()
}
