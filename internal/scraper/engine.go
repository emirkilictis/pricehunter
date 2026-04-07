// Package scraper provides a modular scraping engine with site-specific
// CSS selector definitions for extracting product name and price.
package scraper

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/emirkilic/pricehunter/internal/client"
	"github.com/temoto/robotstxt"
)

// SiteConfig defines CSS selectors for name and price extraction per site.
type SiteConfig struct {
	Domain         string
	NameSelectors  []string
	PriceSelectors []string
	PriceAttr      string // Optional: attribute name to read price from (e.g., "content")
}

// ScrapeResult holds the extracted data from a single scrape.
type ScrapeResult struct {
	URL       string
	Name      string
	Price     float64
	Currency  string
	ImageURL  string
	ScrapedAt time.Time
	Error     error
}

// Engine is the core scraper that handles multiple e-commerce sites.
type Engine struct {
	client          *client.StealthClient
	browser         *BrowserScraper
	sites           map[string]*SiteConfig
	respectRobots   bool
	robotsCache     map[string]*robotstxt.RobotsData
	robotsCacheMu   sync.RWMutex
	requestDelayMs  int
}

// NewEngine creates a new scraper engine with the given stealth client.
func NewEngine(c *client.StealthClient, respectRobots bool, requestDelayMs int) *Engine {
	e := &Engine{
		client:         c,
		sites:          make(map[string]*SiteConfig),
		respectRobots:  respectRobots,
		robotsCache:    make(map[string]*robotstxt.RobotsData),
		requestDelayMs: requestDelayMs,
	}
	e.registerDefaultSites()

	// Initialize headless browser for JS-rendered sites
	e.browser = NewBrowserScraper()

	return e
}

// Close shuts down the engine and its browser instance.
func (e *Engine) Close() {
	if e.browser != nil {
		e.browser.Close()
	}
}

// registerDefaultSites configures CSS selectors for known Turkish e-commerce sites.
func (e *Engine) registerDefaultSites() {
	// Trendyol
	e.sites["trendyol.com"] = &SiteConfig{
		Domain: "trendyol.com",
		NameSelectors: []string{
			"h1.pr-new-br span",
			"h1.pr-new-br",
			".product-title",
			"h1[class*='title']",
			"meta[property='og:title']",
		},
		PriceSelectors: []string{
			"span.prc-dsc",
			"span.prc-slg",
			".product-price-container span.prc-dsc",
			"meta[property='product:price:amount']",
			"span[data-testid='price-current-price']",
		},
	}

	// Hepsiburada
	e.sites["hepsiburada.com"] = &SiteConfig{
		Domain: "hepsiburada.com",
		NameSelectors: []string{
			"h1#product-name",
			"h1[data-test-id='product-name']",
			".product-name",
			"meta[property='og:title']",
		},
		PriceSelectors: []string{
			"span[data-test-id='price-current-price']",
			"span[id='offering-price']",
			".price-value",
			"meta[property='product:price:amount']",
			"span.product-price",
		},
	}

	// Amazon.com.tr
	e.sites["amazon.com.tr"] = &SiteConfig{
		Domain: "amazon.com.tr",
		NameSelectors: []string{
			"#productTitle",
			"span#productTitle",
			"meta[property='og:title']",
		},
		PriceSelectors: []string{
			"span.a-price-whole",
			".a-price .a-offscreen",
			"#priceblock_ourprice",
			"#priceblock_dealprice",
			"span[data-a-color='price'] .a-offscreen",
			"meta[property='product:price:amount']",
		},
	}
}

// RegisterSite adds or updates a site configuration.
func (e *Engine) RegisterSite(cfg *SiteConfig) {
	e.sites[cfg.Domain] = cfg
}

// matchDomain finds the site config that matches the given URL.
func (e *Engine) matchDomain(url string) *SiteConfig {
	lower := strings.ToLower(url)
	for domain, cfg := range e.sites {
		if strings.Contains(lower, domain) {
			return cfg
		}
	}
	return nil
}

// checkRobots verifies if the URL is allowed by the site's robots.txt.
func (e *Engine) checkRobots(ctx context.Context, targetURL string) (bool, error) {
	if !e.respectRobots {
		return true, nil
	}

	// Extract base URL for robots.txt
	parts := strings.SplitN(targetURL, "//", 2)
	if len(parts) < 2 {
		return true, nil
	}
	hostParts := strings.SplitN(parts[1], "/", 2)
	baseURL := parts[0] + "//" + hostParts[0]
	robotsURL := baseURL + "/robots.txt"

	e.robotsCacheMu.RLock()
	data, cached := e.robotsCache[baseURL]
	e.robotsCacheMu.RUnlock()

	if !cached {
		resp, err := e.client.Do(ctx, robotsURL)
		if err != nil {
			// If we can't fetch robots.txt, allow the request
			return true, nil
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return true, nil
		}

		data, err = robotstxt.FromBytes(body)
		if err != nil {
			return true, nil
		}

		e.robotsCacheMu.Lock()
		e.robotsCache[baseURL] = data
		e.robotsCacheMu.Unlock()
	}

	path := "/"
	if len(parts) > 1 {
		pathParts := strings.SplitN(parts[1], "/", 2)
		if len(pathParts) > 1 {
			path = "/" + pathParts[1]
		}
	}

	group := data.FindGroup("*")
	if group == nil {
		return true, nil
	}

	return group.Test(path), nil
}

// Scrape fetches the given URL and extracts product name and price.
func (e *Engine) Scrape(ctx context.Context, targetURL string) *ScrapeResult {
	result := &ScrapeResult{
		URL:       targetURL,
		ScrapedAt: time.Now(),
	}

	// Check robots.txt
	allowed, err := e.checkRobots(ctx, targetURL)
	if err != nil {
		result.Error = fmt.Errorf("robots check failed: %w", err)
		return result
	}
	if !allowed {
		result.Error = fmt.Errorf("blocked by robots.txt: %s", targetURL)
		return result
	}

	// Rate limiting
	if e.requestDelayMs > 0 {
		time.Sleep(time.Duration(e.requestDelayMs) * time.Millisecond)
	}

	// Use headless browser for JS-rendered SPA sites
	if NeedsBrowser(targetURL) && e.browser != nil && e.browser.IsEnabled() {
		siteCfg := e.matchDomain(targetURL)
		browserResult := e.browser.Scrape(ctx, targetURL, siteCfg)
		if browserResult.Error == nil && (browserResult.Name != "" || browserResult.Price > 0) {
			return browserResult
		}
		// If browser scraping failed, fall through to HTTP scraping
		log.Printf("[Engine] Browser scrape failed for %s, trying HTTP: %v", targetURL, browserResult.Error)
	}

	siteCfg := e.matchDomain(targetURL)
	if siteCfg == nil {
		// Use generic selectors for unknown sites
		siteCfg = &SiteConfig{
			NameSelectors: []string{
				"h1",
				"meta[property='og:title']",
				"title",
			},
			PriceSelectors: []string{
				"meta[property='product:price:amount']",
				"[itemprop='price']",
				".price",
			},
		}
	}

	resp, err := e.client.Do(ctx, targetURL)
	if err != nil {
		result.Error = fmt.Errorf("HTTP request failed: %w", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Errorf("HTTP %d for %s", resp.StatusCode, targetURL)
		return result
	}

	// Handle gzip encoding
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			result.Error = fmt.Errorf("gzip decode failed: %w", err)
			return result
		}
		defer gzReader.Close()
		reader = gzReader
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		result.Error = fmt.Errorf("HTML parse failed: %w", err)
		return result
	}

	// Strategy 1: Try CSS selectors first
	result.Name = e.extractText(doc, siteCfg.NameSelectors)
	priceStr := e.extractText(doc, siteCfg.PriceSelectors)
	if priceStr != "" {
		result.Price = parsePrice(priceStr)
		result.Currency = "TRY"
	}

	// Strategy 2: Try JSON-LD structured data (works for many SPA sites)
	if result.Name == "" || result.Price == 0 {
		e.extractFromJSONLD(doc, result)
	}

	// Strategy 3: Try meta tags as final fallback
	if result.Name == "" {
		if val, exists := doc.Find("meta[property='og:title']").Attr("content"); exists {
			result.Name = strings.TrimSpace(val)
		}
	}
	if result.Price == 0 {
		if val, exists := doc.Find("meta[property='product:price:amount']").Attr("content"); exists {
			result.Price = parsePrice(strings.TrimSpace(val))
			result.Currency = "TRY"
		}
	}

	// Extract product image - try multiple strategies
	// Strategy 1: og:image meta tag
	ogImage, exists := doc.Find("meta[property='og:image']").Attr("content")
	if exists && strings.TrimSpace(ogImage) != "" {
		result.ImageURL = cleanImageURL(strings.TrimSpace(ogImage))
	}

	// Strategy 2: Amazon-specific image selectors (Amazon doesn't use og:image consistently)
	if result.ImageURL == "" {
		amazonImgSelectors := []string{
			"#landingImage",
			"#imgBlkFront",
			"#main-image",
			"img.a-dynamic-image",
			"#imgTagWrapperId img",
		}
		for _, sel := range amazonImgSelectors {
			if src, exists := doc.Find(sel).Attr("src"); exists && strings.HasPrefix(src, "http") {
				result.ImageURL = cleanImageURL(strings.TrimSpace(src))
				break
			}
			// Also try data-old-hires for high-res version
			if src, exists := doc.Find(sel).Attr("data-old-hires"); exists && strings.HasPrefix(src, "http") {
				result.ImageURL = cleanImageURL(strings.TrimSpace(src))
				break
			}
		}
	}

	// Strategy 3: Generic product image selectors
	if result.ImageURL == "" {
		genericImgSelectors := []string{
			"meta[property='product:image']",
			"meta[name='twitter:image']",
		}
		for _, sel := range genericImgSelectors {
			if val, exists := doc.Find(sel).Attr("content"); exists && strings.HasPrefix(val, "http") {
				result.ImageURL = cleanImageURL(strings.TrimSpace(val))
				break
			}
		}
	}

	if result.Name == "" && result.Price == 0 {
		result.Error = fmt.Errorf("could not extract data from %s", targetURL)
	}

	return result
}

// extractFromJSONLD parses JSON-LD structured data from script tags.
// Many e-commerce sites embed Product schema even in SPA-rendered pages.
func (e *Engine) extractFromJSONLD(doc *goquery.Document, result *ScrapeResult) {
	doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}

		// Try to find product-related JSON-LD
		// Look for "name" and "price" or "offers" fields
		if strings.Contains(text, `"@type"`) && (strings.Contains(text, `"Product"`) || strings.Contains(text, `"product"`)) {
			// Extract name
			if result.Name == "" {
				if name := extractJSONField(text, "name"); name != "" {
					result.Name = name
				}
			}

			// Extract price from offers
			if result.Price == 0 {
				if price := extractJSONField(text, "price"); price != "" {
					result.Price = parsePrice(price)
					result.Currency = "TRY"
				} else if lowPrice := extractJSONField(text, "lowPrice"); lowPrice != "" {
					result.Price = parsePrice(lowPrice)
					result.Currency = "TRY"
				}
			}

			// Extract image
			if result.ImageURL == "" {
				if img := extractJSONField(text, "image"); img != "" {
					result.ImageURL = cleanImageURL(img)
				}
			}
		}
	})
}

// extractJSONField extracts a simple string value from a JSON string by field name.
// This is a lightweight approach that avoids full JSON unmarshaling for resilience.
func extractJSONField(jsonStr, field string) string {
	// Pattern: "field":"value" or "field": "value"
	patterns := []string{
		`"` + field + `":"`,
		`"` + field + `": "`,
		`"` + field + `" : "`,
	}

	for _, pattern := range patterns {
		idx := strings.Index(jsonStr, pattern)
		if idx == -1 {
			continue
		}
		start := idx + len(pattern)
		end := strings.Index(jsonStr[start:], `"`)
		if end == -1 {
			continue
		}
		val := jsonStr[start : start+end]
		val = strings.ReplaceAll(val, `\/`, `/`)
		return val
	}

	// Also try numeric values: "field":12345 or "field": 12345
	numPatterns := []string{
		`"` + field + `":`,
		`"` + field + `": `,
	}
	for _, pattern := range numPatterns {
		idx := strings.Index(jsonStr, pattern)
		if idx == -1 {
			continue
		}
		start := idx + len(pattern)
		// Read until comma, brace or bracket
		end := start
		for end < len(jsonStr) && jsonStr[end] != ',' && jsonStr[end] != '}' && jsonStr[end] != ']' && jsonStr[end] != ' ' {
			end++
		}
		val := strings.TrimSpace(jsonStr[start:end])
		val = strings.Trim(val, `"`)
		if val != "" && val != "null" {
			return val
		}
	}

	return ""
}

// extractText tries multiple CSS selectors and returns the first non-empty match.
func (e *Engine) extractText(doc *goquery.Document, selectors []string) string {
	for _, sel := range selectors {
		// Handle meta tags
		if strings.HasPrefix(sel, "meta[") {
			val, exists := doc.Find(sel).Attr("content")
			if exists && strings.TrimSpace(val) != "" {
				return strings.TrimSpace(val)
			}
			continue
		}

		text := strings.TrimSpace(doc.Find(sel).First().Text())
		if text != "" {
			return text
		}

		// Try attribute-based extraction
		if val, exists := doc.Find(sel).First().Attr("content"); exists && val != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

// priceRegex matches digits, dots, and commas in a price string.
var priceRegex = regexp.MustCompile(`[\d.,]+`)

// cleanImageURL extracts a clean URL from potentially malformed image values.
// Some sites (Hepsiburada) return JSON arrays like ["https://...jpg"] as og:image content.
func cleanImageURL(raw string) string {
	// Strip JSON array brackets and quotes
	raw = strings.TrimLeft(raw, "[ \"")
	raw = strings.TrimRight(raw, "] \"")

	// If it contains multiple URLs, take the first one
	if idx := strings.Index(raw, "\""); idx > 0 {
		raw = raw[:idx]
	}
	if idx := strings.Index(raw, ","); idx > 0 && !strings.HasPrefix(raw, "http") {
		raw = raw[:idx]
	}

	// Make sure it's actually a URL
	if strings.HasPrefix(raw, "http") {
		return raw
	}
	return ""
}

// parsePrice converts a price string to float64.
// Handles both Turkish format "12.345,99 TL" and international "12345.99".
func parsePrice(s string) float64 {
	match := priceRegex.FindString(s)
	if match == "" {
		return 0
	}

	hasComma := strings.Contains(match, ",")
	hasDot := strings.Contains(match, ".")

	var cleaned string

	if hasComma {
		// Turkish format: 12.345,99 → dots are thousands separators, comma is decimal
		cleaned = strings.ReplaceAll(match, ".", "")
		cleaned = strings.Replace(cleaned, ",", ".", 1)
	} else if hasDot {
		// Could be international decimal (58049.00) or Turkish thousands (58.049)
		dotParts := strings.Split(match, ".")
		lastPart := dotParts[len(dotParts)-1]

		if len(dotParts) == 2 && len(lastPart) <= 2 {
			// International decimal format: 58049.00 or 12345.5
			cleaned = match
		} else if len(dotParts) == 2 && len(lastPart) == 3 {
			// Ambiguous: could be 58.049 (Turkish thousands) or less likely a decimal
			// Treat as Turkish thousands separator since 3 digits after dot
			cleaned = strings.ReplaceAll(match, ".", "")
		} else {
			// Multiple dots like 12.345.678 → dots are thousands separators
			cleaned = strings.ReplaceAll(match, ".", "")
		}
	} else {
		// Pure number no separators
		cleaned = match
	}

	price, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0
	}
	return price
}
