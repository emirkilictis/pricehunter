// Package scraper - browser.go provides headless Chrome scraping
// for JavaScript-rendered SPA sites (Trendyol, Hepsiburada).
package scraper

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

// BrowserScraper uses headless Chrome to render JavaScript-heavy pages.
type BrowserScraper struct {
	allocCtx context.Context
	cancel   context.CancelFunc
	enabled  bool
}

// NewBrowserScraper creates a headless Chrome context for JS rendering.
func NewBrowserScraper() *BrowserScraper {
	bs := &BrowserScraper{}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("disable-notifications", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("block-new-web-contents", true),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	// Test if Chrome is available
	testCtx, testCancel := chromedp.NewContext(allocCtx)
	testCtx, deadlineCancel := context.WithTimeout(testCtx, 10*time.Second)

	err := chromedp.Run(testCtx, chromedp.Navigate("about:blank"))
	deadlineCancel()
	testCancel()

	if err != nil {
		log.Printf("[BrowserScraper] ⚠️  Chrome not available, browser scraping disabled: %v", err)
		allocCancel()
		bs.enabled = false
		return bs
	}

	bs.allocCtx = allocCtx
	bs.cancel = allocCancel
	bs.enabled = true
	log.Printf("[BrowserScraper] ✅ Headless Chrome initialized")
	return bs
}

// IsEnabled returns whether the browser scraper is available.
func (bs *BrowserScraper) IsEnabled() bool {
	return bs.enabled
}

// Close shuts down the Chrome instance.
func (bs *BrowserScraper) Close() {
	if bs.cancel != nil {
		bs.cancel()
	}
}

// NeedsBrowser checks if the given URL requires JavaScript rendering.
func NeedsBrowser(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "trendyol.com") ||
		strings.Contains(lower, "hepsiburada.com")
}

// Scrape renders a page with headless Chrome and extracts product data.
func (bs *BrowserScraper) Scrape(ctx context.Context, targetURL string, siteCfg *SiteConfig) *ScrapeResult {
	result := &ScrapeResult{
		URL:       targetURL,
		ScrapedAt: time.Now(),
	}

	if !bs.enabled {
		result.Error = fmt.Errorf("browser scraper not available")
		return result
	}

	// Create a new tab context
	tabCtx, tabCancel := chromedp.NewContext(bs.allocCtx)
	defer tabCancel()

	// Set a timeout for the whole operation
	tabCtx, deadlineCancel := context.WithTimeout(tabCtx, 30*time.Second)
	defer deadlineCancel()

	var htmlContent string

	// Navigate and wait for content to render
	err := chromedp.Run(tabCtx,
		chromedp.Navigate(targetURL),
		// Wait for the page to load
		chromedp.Sleep(3*time.Second),
		// Try to wait for a common product element
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Wait for body to be populated
			return chromedp.WaitVisible("body", chromedp.ByQuery).Do(ctx)
		}),
		// Additional wait for dynamic content
		chromedp.Sleep(2*time.Second),
		// Get the full rendered HTML
		chromedp.OuterHTML("html", &htmlContent),
	)

	if err != nil {
		result.Error = fmt.Errorf("browser render failed: %w", err)
		return result
	}

	// Parse the rendered HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		result.Error = fmt.Errorf("HTML parse failed after render: %w", err)
		return result
	}

	// Extract product name using CSS selectors
	if siteCfg != nil {
		for _, sel := range siteCfg.NameSelectors {
			if strings.HasPrefix(sel, "meta[") {
				if val, exists := doc.Find(sel).Attr("content"); exists && strings.TrimSpace(val) != "" {
					result.Name = strings.TrimSpace(val)
					break
				}
				continue
			}
			text := strings.TrimSpace(doc.Find(sel).First().Text())
			if text != "" {
				result.Name = text
				break
			}
		}

		// Extract price
		for _, sel := range siteCfg.PriceSelectors {
			if strings.HasPrefix(sel, "meta[") {
				if val, exists := doc.Find(sel).Attr("content"); exists && strings.TrimSpace(val) != "" {
					result.Price = parsePrice(strings.TrimSpace(val))
					result.Currency = "TRY"
					break
				}
				continue
			}
			text := strings.TrimSpace(doc.Find(sel).First().Text())
			if text != "" {
				result.Price = parsePrice(text)
				result.Currency = "TRY"
				break
			}
		}
	}

	// Fallback: Try JSON-LD from rendered page
	if result.Name == "" || result.Price == 0 {
		extractFromJSONLDHelper(doc, result)
	}

	// Fallback: Try meta tags
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

	// Extract image
	if ogImg, exists := doc.Find("meta[property='og:image']").Attr("content"); exists {
		result.ImageURL = cleanImageURL(strings.TrimSpace(ogImg))
	}

	if result.Name == "" && result.Price == 0 {
		result.Error = fmt.Errorf("could not extract data from %s (browser render)", targetURL)
	}

	return result
}

// extractFromJSONLDHelper is a standalone helper for browser.go to use the same JSON-LD logic.
func extractFromJSONLDHelper(doc *goquery.Document, result *ScrapeResult) {
	doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}
		if strings.Contains(text, `"@type"`) && (strings.Contains(text, `"Product"`) || strings.Contains(text, `"product"`)) {
			if result.Name == "" {
				if name := extractJSONField(text, "name"); name != "" {
					result.Name = name
				}
			}
			if result.Price == 0 {
				if price := extractJSONField(text, "price"); price != "" {
					result.Price = parsePrice(price)
					result.Currency = "TRY"
				} else if lowPrice := extractJSONField(text, "lowPrice"); lowPrice != "" {
					result.Price = parsePrice(lowPrice)
					result.Currency = "TRY"
				}
			}
			if result.ImageURL == "" {
				if img := extractJSONField(text, "image"); img != "" {
					result.ImageURL = cleanImageURL(img)
				}
			}
		}
	})
}
