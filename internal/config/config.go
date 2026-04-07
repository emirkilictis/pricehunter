// Package config provides configuration loading and management for PriceHunter.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Config represents the application configuration loaded from config.json.
type Config struct {
	ScrapeIntervalMin     int            `json:"scrape_interval_minutes"`
	RequestDelayMs        int            `json:"request_delay_ms"`
	MaxWorkers            int            `json:"max_workers"`
	RequestTimeoutSeconds int            `json:"request_timeout_seconds"`
	RespectRobotsTxt      bool           `json:"respect_robots_txt"`
	Proxies               []string       `json:"proxies"`
	UserAgents            []string       `json:"user_agents"`
	Notification          NotificationCfg `json:"notification"`
	API                   APICfg          `json:"api"`
	Products              []ProductCfg    `json:"products"`
}

// NotificationCfg holds notification channel settings.
type NotificationCfg struct {
	Enabled                 bool    `json:"enabled"`
	DiscordWebhookURL       string  `json:"discord_webhook_url"`
	TelegramBotToken        string  `json:"telegram_bot_token"`
	TelegramChatID          string  `json:"telegram_chat_id"`
	PriceDropThresholdPct   float64 `json:"price_drop_threshold_percent"`
}

// APICfg holds REST API server settings.
type APICfg struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// ProductCfg represents a product to track from the config file.
type ProductCfg struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

var (
	instance *Config
	once     sync.Once
	loadErr  error
)

// Load reads configuration from the given file path. Thread-safe with sync.Once.
func Load(path string) (*Config, error) {
	once.Do(func() {
		data, err := os.ReadFile(path)
		if err != nil {
			loadErr = fmt.Errorf("config: failed to read %s: %w", path, err)
			return
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			loadErr = fmt.Errorf("config: failed to parse %s: %w", path, err)
			return
		}

		// Apply defaults
		if cfg.MaxWorkers <= 0 {
			cfg.MaxWorkers = 3
		}
		if cfg.ScrapeIntervalMin <= 0 {
			cfg.ScrapeIntervalMin = 30
		}
		if cfg.RequestDelayMs <= 0 {
			cfg.RequestDelayMs = 2000
		}
		if cfg.RequestTimeoutSeconds <= 0 {
			cfg.RequestTimeoutSeconds = 15
		}
		if cfg.API.Port <= 0 {
			cfg.API.Port = 8080
		}
		if cfg.API.Host == "" {
			cfg.API.Host = "0.0.0.0"
		}
		if len(cfg.UserAgents) == 0 {
			cfg.UserAgents = []string{
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
			}
		}

		instance = &cfg
	})

	return instance, loadErr
}

// Get returns the loaded configuration. Must call Load first.
func Get() *Config {
	return instance
}
