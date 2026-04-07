package scraper

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// SearchResult holds a product found via search on a specific site.
type SearchResult struct {
	Site     string  `json:"site"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	URL      string  `json:"url"`
	ImageURL string  `json:"image_url"`
	Error    string  `json:"error,omitempty"`
}

// SearchAcrossSites searches for a query across all supported e-commerce sites in parallel.
func (e *Engine) SearchAcrossSites(ctx context.Context, query string) []SearchResult {
	sites := []struct {
		name string
		fn   func(context.Context, string) (*SearchResult, error)
	}{
		{"Trendyol", e.searchTrendyol},
		{"Hepsiburada", e.searchHepsiburada},
		{"Amazon TR", e.searchAmazon},
	}

	results := make([]SearchResult, 0, len(sites))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, s := range sites {
		wg.Add(1)
		go func(siteName string, fn func(context.Context, string) (*SearchResult, error)) {
			defer wg.Done()
			r, err := fn(ctx, query)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				log.Printf("[Search] %s error: %v", siteName, err)
				results = append(results, SearchResult{Site: siteName, Error: err.Error()})
				return
			}
			if r != nil {
				results = append(results, *r)
			}
		}(s.name, s.fn)
	}

	wg.Wait()
	return results
}

// searchTrendyol searches Trendyol and returns the first product result.
func (e *Engine) searchTrendyol(ctx context.Context, query string) (*SearchResult, error) {
	searchURL := "https://www.trendyol.com/sr?q=" + url.QueryEscape(query)

	doc, err := e.fetchDoc(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	var result *SearchResult

	// Try product cards in search results
	doc.Find(".p-card-wrppr, [data-testid='product-card']").Each(func(i int, s *goquery.Selection) {
		if result != nil || i > 0 {
			return
		}

		name := strings.TrimSpace(s.Find(".prdct-desc-cntnr-name").Text())
		if name == "" {
			name = strings.TrimSpace(s.Find("[title]").AttrOr("title", ""))
		}
		if name == "" {
			name = strings.TrimSpace(s.Find("h3").Text())
		}

		priceText := strings.TrimSpace(s.Find(".prc-box-dscntd, .prc-box-sllng").First().Text())
		price := parsePrice(priceText)

		href, _ := s.Find("a").First().Attr("href")
		if href != "" && !strings.HasPrefix(href, "http") {
			href = "https://www.trendyol.com" + href
		}

		imgURL := ""
		s.Find("img").Each(func(_ int, img *goquery.Selection) {
			if imgURL != "" {
				return
			}
			for _, attr := range []string{"src", "data-src", "data-original"} {
				if v, ok := img.Attr(attr); ok && strings.HasPrefix(v, "http") {
					imgURL = v
					return
				}
			}
		})

		if name != "" && href != "" {
			result = &SearchResult{
				Site:     "Trendyol",
				Name:     name,
				Price:    price,
				Currency: "TRY",
				URL:      href,
				ImageURL: imgURL,
			}
		}
	})

	if result == nil {
		return nil, fmt.Errorf("no results found on Trendyol")
	}
	return result, nil
}

// searchHepsiburada searches Hepsiburada and returns the first product result.
func (e *Engine) searchHepsiburada(ctx context.Context, query string) (*SearchResult, error) {
	searchURL := "https://www.hepsiburada.com/ara?q=" + url.QueryEscape(query)

	doc, err := e.fetchDoc(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	var result *SearchResult

	doc.Find("[data-test-id='product-card-wrapper'], li[class*='productListContent']").Each(func(i int, s *goquery.Selection) {
		if result != nil || i > 0 {
			return
		}

		name := strings.TrimSpace(s.Find("[data-test-id='product-card-name'], h3, .product-name").First().Text())
		priceText := strings.TrimSpace(s.Find("[data-test-id='product-card-price'], span.price-value, .product-price").First().Text())
		price := parsePrice(priceText)

		href, _ := s.Find("a").First().Attr("href")
		if href != "" && !strings.HasPrefix(href, "http") {
			href = "https://www.hepsiburada.com" + href
		}

		imgURL := ""
		if img := s.Find("img").First(); img != nil {
			for _, attr := range []string{"src", "data-src"} {
				if v, ok := img.Attr(attr); ok && strings.HasPrefix(v, "http") {
					imgURL = v
					break
				}
			}
		}

		if name != "" && href != "" {
			result = &SearchResult{
				Site:     "Hepsiburada",
				Name:     name,
				Price:    price,
				Currency: "TRY",
				URL:      href,
				ImageURL: imgURL,
			}
		}
	})

	if result == nil {
		return nil, fmt.Errorf("no results found on Hepsiburada")
	}
	return result, nil
}

// searchAmazon searches Amazon TR and returns the first product result.
func (e *Engine) searchAmazon(ctx context.Context, query string) (*SearchResult, error) {
	searchURL := "https://www.amazon.com.tr/s?k=" + url.QueryEscape(query)

	doc, err := e.fetchDoc(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	var result *SearchResult

	doc.Find("[data-component-type='s-search-result']").Each(func(i int, s *goquery.Selection) {
		if result != nil || i > 2 { // Try first 3 results for Amazon (skip sponsored)
			return
		}

		// Skip sponsored
		if s.Find(".s-label-popover-default").Length() > 0 {
			return
		}

		name := strings.TrimSpace(s.Find("span.a-text-normal, h2 a span").First().Text())
		priceText := ""
		s.Find("span.a-price-whole").Each(func(_ int, p *goquery.Selection) {
			if priceText == "" {
				priceText = strings.TrimSpace(p.Text())
			}
		})
		price := parsePrice(priceText)

		href, _ := s.Find("a.a-link-normal[href]").First().Attr("href")
		if href != "" && !strings.HasPrefix(href, "http") {
			href = "https://www.amazon.com.tr" + href
		}

		imgURL := ""
		if img := s.Find("img.s-image").First(); img.Length() > 0 {
			imgURL, _ = img.Attr("src")
		}

		if name != "" && href != "" {
			result = &SearchResult{
				Site:     "Amazon TR",
				Name:     name,
				Price:    price,
				Currency: "TRY",
				URL:      href,
				ImageURL: imgURL,
			}
		}
	})

	if result == nil {
		return nil, fmt.Errorf("no results found on Amazon TR")
	}
	return result, nil
}

// fetchDoc fetches and parses an HTML page using the stealth client.
func (e *Engine) fetchDoc(ctx context.Context, targetURL string) (*goquery.Document, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	resp, err := e.client.Do(timeoutCtx, targetURL)
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}
	return doc, nil
}
