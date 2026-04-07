// Package storage provides SQLite-backed persistence for products and price history.
package storage

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Product represents a tracked product.
type Product struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	Name      string    `json:"name"`
	Site      string    `json:"site"`
	ImageURL  string    `json:"image_url"`
	LastPrice float64   `json:"last_price"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PriceRecord represents a single price observation.
type PriceRecord struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"product_id"`
	Price     float64   `json:"price"`
	Currency  string    `json:"currency"`
	ScrapedAt time.Time `json:"scraped_at"`
}

// PriceChange represents a detected price difference.
type PriceChange struct {
	ProductID int64   `json:"product_id"`
	Product   string  `json:"product_name"`
	OldPrice  float64 `json:"old_price"`
	NewPrice  float64 `json:"new_price"`
	ChangePct float64 `json:"change_pct"`
}

// DB wraps an SQLite connection with thread-safe operations.
type DB struct {
	conn *sql.DB
	mu   sync.RWMutex
}

// NewDB opens or creates an SQLite database and initializes the schema.
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_timeout=5000&_fk=true")
	if err != nil {
		return nil, fmt.Errorf("storage: failed to open database: %w", err)
	}

	// Connection pool settings for concurrent access
	conn.SetMaxOpenConns(1) // SQLite is single-writer
	conn.SetMaxIdleConns(1)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("storage: migration failed: %w", err)
	}

	return db, nil
}

// migrate creates the required tables if they don't exist.
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		site TEXT NOT NULL DEFAULT '',
		image_url TEXT NOT NULL DEFAULT '',
		last_price REAL NOT NULL DEFAULT 0,
		currency TEXT NOT NULL DEFAULT 'TRY',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS price_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		product_id INTEGER NOT NULL,
		price REAL NOT NULL,
		currency TEXT NOT NULL DEFAULT 'TRY',
		scraped_at DATETIME NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_price_history_product_id ON price_history(product_id);
	CREATE INDEX IF NOT EXISTS idx_price_history_scraped_at ON price_history(scraped_at);
	CREATE INDEX IF NOT EXISTS idx_products_url ON products(url);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// UpsertProduct inserts or updates a product. Returns the product ID and any price change.
func (db *DB) UpsertProduct(url, name, site, imageURL string, price float64, currency string) (int64, *PriceChange, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Check if product exists
	var existing Product
	err := db.conn.QueryRow(
		"SELECT id, name, last_price FROM products WHERE url = ?", url,
	).Scan(&existing.ID, &existing.Name, &existing.LastPrice)

	var productID int64
	var change *PriceChange

	if err == sql.ErrNoRows {
		// Insert new product
		res, err := db.conn.Exec(
			`INSERT INTO products (url, name, site, image_url, last_price, currency, created_at, updated_at) 
			 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
			url, name, site, imageURL, price, currency,
		)
		if err != nil {
			return 0, nil, fmt.Errorf("storage: insert product failed: %w", err)
		}
		productID, _ = res.LastInsertId()
	} else if err != nil {
		return 0, nil, fmt.Errorf("storage: query product failed: %w", err)
	} else {
		productID = existing.ID

		// Update product info
		displayName := name
		if displayName == "" {
			displayName = existing.Name
		}

		_, err = db.conn.Exec(
			`UPDATE products SET name = ?, image_url = CASE WHEN ? != '' THEN ? ELSE image_url END, last_price = ?, currency = ?, updated_at = datetime('now') WHERE id = ?`,
			displayName, imageURL, imageURL, price, currency, productID,
		)
		if err != nil {
			return 0, nil, fmt.Errorf("storage: update product failed: %w", err)
		}

		// Detect price change
		if existing.LastPrice > 0 && price > 0 && existing.LastPrice != price {
			pct := ((price - existing.LastPrice) / existing.LastPrice) * 100
			change = &PriceChange{
				ProductID: productID,
				Product:   displayName,
				OldPrice:  existing.LastPrice,
				NewPrice:  price,
				ChangePct: pct,
			}
		}
	}

	// Record price history
	if price > 0 {
		_, err = db.conn.Exec(
			`INSERT INTO price_history (product_id, price, currency, scraped_at) VALUES (?, ?, ?, datetime('now'))`,
			productID, price, currency,
		)
		if err != nil {
			return productID, change, fmt.Errorf("storage: insert price record failed: %w", err)
		}
	}

	return productID, change, nil
}

// GetAllProducts returns all tracked products.
func (db *DB) GetAllProducts() ([]Product, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.conn.Query(
		`SELECT id, url, name, site, image_url, last_price, currency, created_at, updated_at 
		 FROM products ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("storage: query products failed: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.URL, &p.Name, &p.Site, &p.ImageURL, &p.LastPrice, &p.Currency, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan product failed: %w", err)
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

// GetProduct returns a single product by ID.
func (db *DB) GetProduct(id int64) (*Product, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var p Product
	err := db.conn.QueryRow(
		`SELECT id, url, name, site, image_url, last_price, currency, created_at, updated_at 
		 FROM products WHERE id = ?`, id,
	).Scan(&p.ID, &p.URL, &p.Name, &p.Site, &p.ImageURL, &p.LastPrice, &p.Currency, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: get product failed: %w", err)
	}
	return &p, nil
}

// GetPriceHistory returns the price history for a product ID.
func (db *DB) GetPriceHistory(productID int64, limit int) ([]PriceRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := db.conn.Query(
		`SELECT id, product_id, price, currency, scraped_at 
		 FROM price_history WHERE product_id = ? ORDER BY scraped_at ASC LIMIT ?`,
		productID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("storage: query price history failed: %w", err)
	}
	defer rows.Close()

	var records []PriceRecord
	for rows.Next() {
		var r PriceRecord
		if err := rows.Scan(&r.ID, &r.ProductID, &r.Price, &r.Currency, &r.ScrapedAt); err != nil {
			return nil, fmt.Errorf("storage: scan price record failed: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// AddProduct adds a new product to track.
func (db *DB) AddProduct(url, name, site string) (int64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	res, err := db.conn.Exec(
		`INSERT OR IGNORE INTO products (url, name, site, created_at, updated_at) 
		 VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		url, name, site,
	)
	if err != nil {
		return 0, fmt.Errorf("storage: add product failed: %w", err)
	}

	id, _ := res.LastInsertId()
	if id == 0 {
		// Product already existed, get its ID
		err = db.conn.QueryRow("SELECT id FROM products WHERE url = ?", url).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("storage: get existing product id failed: %w", err)
		}
	}
	return id, nil
}

// DeleteProduct removes a product and its history.
func (db *DB) DeleteProduct(id int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec("DELETE FROM products WHERE id = ?", id)
	return err
}

// PriceStats contains statistical analysis of a product's price history.
type PriceStats struct {
	MinPrice     float64   `json:"min_price"`
	MaxPrice     float64   `json:"max_price"`
	AvgPrice     float64   `json:"avg_price"`
	CurrentPrice float64   `json:"current_price"`
	DataPoints   int       `json:"data_points"`
	IsAtLowest   bool      `json:"is_at_lowest"`
	IsAtHighest  bool      `json:"is_at_highest"`
	DropFromMax  float64   `json:"drop_from_max"`  // Percentage drop from max
	RiseFromMin  float64   `json:"rise_from_min"`  // Percentage rise from min
	FirstSeen    time.Time `json:"first_seen"`
	LastUpdated  time.Time `json:"last_updated"`
}

// GetPriceStats returns statistical analysis for a product's price history.
func (db *DB) GetPriceStats(productID int64) (*PriceStats, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var stats PriceStats
	var firstSeenStr, lastUpdatedStr string
	err := db.conn.QueryRow(`
		SELECT 
			COALESCE(MIN(price), 0),
			COALESCE(MAX(price), 0),
			COALESCE(AVG(price), 0),
			COUNT(*),
			COALESCE(MIN(scraped_at), ''),
			COALESCE(MAX(scraped_at), '')
		FROM price_history WHERE product_id = ?
	`, productID).Scan(
		&stats.MinPrice, &stats.MaxPrice, &stats.AvgPrice,
		&stats.DataPoints, &firstSeenStr, &lastUpdatedStr,
	)
	if err != nil {
		return nil, fmt.Errorf("storage: get price stats failed: %w", err)
	}

	// Parse datetime strings
	if firstSeenStr != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", firstSeenStr); err == nil {
			stats.FirstSeen = t
		}
	}
	if lastUpdatedStr != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", lastUpdatedStr); err == nil {
			stats.LastUpdated = t
		}
	}

	// Get current price
	err = db.conn.QueryRow(
		"SELECT last_price FROM products WHERE id = ?", productID,
	).Scan(&stats.CurrentPrice)
	if err != nil {
		return nil, fmt.Errorf("storage: get current price failed: %w", err)
	}

	// Calculate flags
	if stats.MinPrice > 0 {
		stats.IsAtLowest = stats.CurrentPrice <= stats.MinPrice
		stats.RiseFromMin = ((stats.CurrentPrice - stats.MinPrice) / stats.MinPrice) * 100
	}
	if stats.MaxPrice > 0 {
		stats.IsAtHighest = stats.CurrentPrice >= stats.MaxPrice
		stats.DropFromMax = ((stats.MaxPrice - stats.CurrentPrice) / stats.MaxPrice) * 100
	}

	return &stats, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}
