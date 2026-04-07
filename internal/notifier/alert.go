// Package notifier provides price drop alerts via terminal logs and webhooks.
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/emirkilic/pricehunter/internal/storage"
)

// Alerter handles price drop notifications.
type Alerter struct {
	discordWebhook  string
	telegramToken   string
	telegramChatID  string
	threshold       float64
	enabled         bool
	httpClient      *http.Client
}

// NewAlerter creates a new alerter with the given notification settings.
func NewAlerter(enabled bool, discordWebhook, telegramToken, telegramChatID string, thresholdPct float64) *Alerter {
	if thresholdPct <= 0 {
		thresholdPct = 5.0
	}
	return &Alerter{
		discordWebhook: discordWebhook,
		telegramToken:  telegramToken,
		telegramChatID: telegramChatID,
		threshold:      thresholdPct,
		enabled:        enabled,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ProcessChange evaluates a price change and sends alerts if the drop exceeds threshold.
func (a *Alerter) ProcessChange(change *storage.PriceChange) {
	if change == nil {
		return
	}

	// Log all price changes to terminal
	direction := "📈"
	if change.ChangePct < 0 {
		direction = "📉"
	}
	log.Printf("[Alert] %s %s: %.2f → %.2f (%+.1f%%)",
		direction, change.Product, change.OldPrice, change.NewPrice, change.ChangePct)

	// Only send external alerts for significant price drops
	if !a.enabled || change.ChangePct >= 0 {
		return
	}

	// Check threshold (negative change means drop)
	dropPct := -change.ChangePct
	if dropPct < a.threshold {
		return
	}

	message := fmt.Sprintf(
		"🔥 **Fiyat Düşüşü Tespit Edildi!**\n\n"+
			"📦 **Ürün:** %s\n"+
			"💰 **Eski Fiyat:** %.2f TL\n"+
			"🏷️ **Yeni Fiyat:** %.2f TL\n"+
			"📉 **Düşüş:** %%%.1f\n"+
			"⏰ **Zaman:** %s",
		change.Product,
		change.OldPrice,
		change.NewPrice,
		dropPct,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	// Discord webhook
	if a.discordWebhook != "" {
		go a.sendDiscord(message)
	}

	// Telegram
	if a.telegramToken != "" && a.telegramChatID != "" {
		go a.sendTelegram(message)
	}
}

// sendDiscord sends a message to a Discord webhook.
func (a *Alerter) sendDiscord(message string) {
	payload := map[string]string{
		"content": message,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[Alert] Discord marshal error: %v", err)
		return
	}

	resp, err := a.httpClient.Post(a.discordWebhook, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("[Alert] Discord send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("[Alert] Discord returned status %d", resp.StatusCode)
	} else {
		log.Printf("[Alert] ✅ Discord notification sent")
	}
}

// sendTelegram sends a message via the Telegram Bot API.
func (a *Alerter) sendTelegram(message string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", a.telegramToken)

	payload := map[string]string{
		"chat_id":    a.telegramChatID,
		"text":       message,
		"parse_mode": "Markdown",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[Alert] Telegram marshal error: %v", err)
		return
	}

	resp, err := a.httpClient.Post(apiURL, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("[Alert] Telegram send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("[Alert] Telegram returned status %d", resp.StatusCode)
	} else {
		log.Printf("[Alert] ✅ Telegram notification sent")
	}
}
