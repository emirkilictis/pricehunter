// Package worker provides a concurrent worker pool for distributed scraping.
package worker

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/emirkilic/pricehunter/internal/notifier"
	"github.com/emirkilic/pricehunter/internal/scraper"
	"github.com/emirkilic/pricehunter/internal/storage"
)

// Job represents a scraping task for a single URL.
type Job struct {
	URL  string
	Name string
}

// Pool manages a fixed number of worker goroutines to process scraping jobs.
type Pool struct {
	workers    int
	engine     *scraper.Engine
	db         *storage.DB
	alerter    *notifier.Alerter
	jobs       chan Job
	wg         sync.WaitGroup
	mu         sync.Mutex
	isRunning  bool
}

// NewPool creates a new worker pool.
func NewPool(workers int, engine *scraper.Engine, db *storage.DB, alerter *notifier.Alerter) *Pool {
	if workers <= 0 {
		workers = 3
	}
	return &Pool{
		workers: workers,
		engine:  engine,
		db:      db,
		alerter: alerter,
		jobs:    make(chan Job, workers*10),
	}
}

// Start launches the worker goroutines.
func (p *Pool) Start(ctx context.Context) {
	p.mu.Lock()
	if p.isRunning {
		p.mu.Unlock()
		return
	}
	p.isRunning = true
	p.mu.Unlock()

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
	log.Printf("[Pool] Started %d workers", p.workers)
}

// Submit adds a job to the pool's queue.
func (p *Pool) Submit(job Job) {
	p.jobs <- job
}

// SubmitBatch adds multiple jobs to the pool's queue.
func (p *Pool) SubmitBatch(jobs []Job) {
	for _, j := range jobs {
		p.Submit(j)
	}
}

// Wait blocks until all submitted jobs are processed.
func (p *Pool) Wait() {
	// Close the job channel, wait for workers to drain
	close(p.jobs)
	p.wg.Wait()

	// Re-open job channel for next batch
	p.mu.Lock()
	p.jobs = make(chan Job, p.workers*10)
	p.isRunning = false
	p.mu.Unlock()
}

// worker is the goroutine that processes jobs from the queue.
func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()

	for job := range p.jobs {
		select {
		case <-ctx.Done():
			log.Printf("[Worker %d] Context cancelled, stopping", id)
			return
		default:
		}

		log.Printf("[Worker %d] Scraping: %s", id, job.URL)
		start := time.Now()

		// Retry logic with exponential backoff
		maxRetries := 2
		var result *scraper.ScrapeResult
		for attempt := 0; attempt <= maxRetries; attempt++ {
			result = p.engine.Scrape(ctx, job.URL)
			if result.Error == nil {
				break
			}
			if attempt < maxRetries {
				delay := time.Duration(1<<uint(attempt+1)) * time.Second // 2s, 4s
				log.Printf("[Worker %d] ⏳ Retry %d/%d for %s in %v...", id, attempt+1, maxRetries, job.URL, delay)
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return
				}
			}
		}

		elapsed := time.Since(start)

		if result.Error != nil {
			log.Printf("[Worker %d] ❌ Error scraping %s after %d retries: %v (took %v)", id, job.URL, maxRetries, result.Error, elapsed)
			continue
		}

		// Use the job name if scraper didn't find one
		name := result.Name
		if name == "" {
			name = job.Name
		}

		// Detect site from URL
		site := detectSite(job.URL)

		// Store result
		_, priceChange, err := p.db.UpsertProduct(job.URL, name, site, result.ImageURL, result.Price, result.Currency)
		if err != nil {
			log.Printf("[Worker %d] ❌ Storage error for %s: %v", id, job.URL, err)
			continue
		}

		log.Printf("[Worker %d] ✅ %s | %s | %.2f %s (took %v)",
			id, site, name, result.Price, result.Currency, elapsed)

		// Send alert if price dropped
		if priceChange != nil && p.alerter != nil {
			p.alerter.ProcessChange(priceChange)
		}
	}
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
	case strings.Contains(lower, "amazon."):
		return "Amazon"
	default:
		return "Other"
	}
}
