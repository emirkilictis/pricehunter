// PriceHunter - Distributed Price Monitor
// A high-performance concurrent web scraping and price analysis system.
//
// Architecture:
//   - Worker Pool: Goroutine-based concurrent scraping with configurable parallelism
//   - Stealth Client: uTLS fingerprinting with proxy and User-Agent rotation
//   - Modular Scraper: Site-specific CSS selectors for Trendyol, Hepsiburada, Amazon TR
//   - SQLite Storage: WAL-mode persistent storage with price history tracking
//   - Notification: Terminal, Discord, and Telegram alerts on price drops
//   - REST API: JSON API serving the React frontend dashboard
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/emirkilic/pricehunter/api"
	"github.com/emirkilic/pricehunter/internal/client"
	"github.com/emirkilic/pricehunter/internal/config"
	"github.com/emirkilic/pricehunter/internal/notifier"
	"github.com/emirkilic/pricehunter/internal/scraper"
	"github.com/emirkilic/pricehunter/internal/storage"
	"github.com/emirkilic/pricehunter/internal/worker"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🔍 PriceHunter - Distributed Price Monitor")
	log.Println("==========================================")

	// --- Load Configuration ---
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}
	log.Printf("✅ Config loaded: %d products, %d workers, %d min interval",
		len(cfg.Products), cfg.MaxWorkers, cfg.ScrapeIntervalMin)

	// --- Initialize Database ---
	db, err := storage.NewDB("pricehunter.db")
	if err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Println("✅ SQLite database initialized (WAL mode)")

	// --- Seed products from config ---
	for _, p := range cfg.Products {
		site := detectSite(p.URL)
		_, err := db.AddProduct(p.URL, p.Name, site)
		if err != nil {
			log.Printf("⚠️  Failed to seed product %s: %v", p.Name, err)
		}
	}

	// --- Initialize Components ---
	stealthClient := client.NewStealthClient(cfg.Proxies, cfg.UserAgents, cfg.RequestTimeoutSeconds)
	log.Printf("✅ Stealth client initialized (uTLS + %d proxies + %d User-Agents)",
		len(cfg.Proxies), len(cfg.UserAgents))

	engine := scraper.NewEngine(stealthClient, cfg.RespectRobotsTxt, cfg.RequestDelayMs)
	log.Println("✅ Scraper engine initialized (Trendyol, Hepsiburada, Amazon TR)")

	alerter := notifier.NewAlerter(
		cfg.Notification.Enabled,
		cfg.Notification.DiscordWebhookURL,
		cfg.Notification.TelegramBotToken,
		cfg.Notification.TelegramChatID,
		cfg.Notification.PriceDropThresholdPct,
	)
	log.Printf("✅ Notification system initialized (enabled=%v, threshold=%.1f%%)",
		cfg.Notification.Enabled, cfg.Notification.PriceDropThresholdPct)

	// --- Context with graceful shutdown ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// --- Start REST API Server ---
	apiServer := api.NewServer(db, engine, cfg.API.Host, cfg.API.Port)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Fatalf("❌ API server failed: %v", err)
		}
	}()

	// --- Scrape Loop ---
	go func() {
		// Run first scrape immediately
		runScrapeRound(ctx, cfg, engine, db, alerter)

		ticker := time.NewTicker(time.Duration(cfg.ScrapeIntervalMin) * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				runScrapeRound(ctx, cfg, engine, db, alerter)
			case <-ctx.Done():
				log.Println("🛑 Scrape loop stopped")
				return
			}
		}
	}()

	// --- Wait for shutdown signal ---
	sig := <-sigChan
	log.Printf("🛑 Received signal %v, shutting down gracefully...", sig)
	cancel()

	// Give workers time to finish
	time.Sleep(2 * time.Second)
	engine.Close()
	log.Println("👋 PriceHunter stopped. Goodbye!")
}

// runScrapeRound performs one full scraping cycle for all tracked products.
func runScrapeRound(ctx context.Context, cfg *config.Config, engine *scraper.Engine, db *storage.DB, alerter *notifier.Alerter) {
	products, err := db.GetAllProducts()
	if err != nil {
		log.Printf("❌ Failed to get products for scraping: %v", err)
		return
	}

	if len(products) == 0 {
		log.Println("⚠️  No products to scrape")
		return
	}

	log.Printf("🔄 Starting scrape round for %d products...", len(products))
	start := time.Now()

	pool := worker.NewPool(cfg.MaxWorkers, engine, db, alerter)
	pool.Start(ctx)

	var jobs []worker.Job
	for _, p := range products {
		jobs = append(jobs, worker.Job{
			URL:  p.URL,
			Name: p.Name,
		})
	}
	pool.SubmitBatch(jobs)
	pool.Wait()

	elapsed := time.Since(start)
	log.Printf("✅ Scrape round completed in %v (%d products)", elapsed, len(products))
}

// detectSite returns a friendly site name from the URL.
func detectSite(url string) string {
	lower := strings.ToLower(url)
	switch {
	case strings.Contains(lower, "trendyol.com"):
		return "Trendyol"
	case strings.Contains(lower, "hepsiburada.com"):
		return "Hepsiburada"
	case strings.Contains(lower, "amazon.com.tr"):
		return "Amazon TR"
	default:
		return "Other"
	}
}
