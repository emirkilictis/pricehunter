# 🎯 PriceHunter

PriceHunter is an advanced, **distributed price monitoring platform** that automatically tracks, analyzes, and sends notifications for products across various e-commerce sites (such as Trendyol, Hepsiburada, and Amazon TR).

It is designed as an end-to-end product, featuring a high-performance backend written in Go and a modern frontend built with React. Its powerful scraping engine is well-equipped to bypass WAFs and anti-bot systems (e.g., Cloudflare, Akamai).

## ✨ Key Features

* **🔎 Smart Product Search & Comparison:** Search for a single product name concurrently across multiple websites and instantly find the cheapest option.
* **🛡️ Advanced Anti-Bot Scraping:** Completely invisible scraping using TLS fingerprinting (`uTLS`), proxy rotation, a User-Agent pool, and an automatically triggered **Headless Chrome (Chromedp)** fallback to prevent bans.
* **📈 Detailed Stats & Charts:** Tracks the highest, lowest, and average historical prices. Visualize the current price position via a fluid gauge meter and interactive price history graphs using `recharts`.
* **⚡ Asynchronous Worker Pool:** Operates a non-blocking `goroutine` worker pool that scrapes all product links simultaneously at your defined interval (e.g., every 30 minutes).
* **🔔 Telegram & Discord Integrations:** Instantly fires a webhook to your devices via Telegram or Discord when a product's price drops below your target percentage (e.g., 5%).

## 🛠️ Tech Stack

**Backend Engine:**
* **Language:** Golang (Go)
* **Database:** SQLite (Configured in `WAL` mode for maximum read/write concurrence and performance)
* **API:** Gorilla Mux (REST JSON API)
* **Scraping Tools:** goquery, chromedp, utls

**Frontend Dashboard:**
* **Framework:** React (Vite)
* **Styling:** Modern Vanilla CSS (Glassmorphism, Pulse animations, Fully Responsive)
* **Charts:** Recharts
* **Icons:** Lucide React

## 🚀 Installation (Local Development)

To run the project on your local machine, you need Go and Node.js installed.

### 1️⃣ Clone the Repository
```bash
git clone https://github.com/emirkilictis/pricehunter.git
cd pricehunter
```

### 2️⃣ Start the Backend (Go)
```bash
# Install dependencies
go mod tidy 

# Build and run the project
go build -o pricehunter .
./pricehunter
```
*The backend API will start running on `http://localhost:8080`.*

### 3️⃣ Start the Frontend (React)
Open a new terminal tab/window and run the following commands:
```bash
cd frontend
npm install
npm run dev
```
*The dashboard will be available at `http://localhost:5173`.*

## ⚙️ Configuration

You can customize the scraping rules, notifications, and proxies via the `config.json` file located in the root directory.

```json
{
  "scrape_interval_minutes": 30,
  "max_workers": 5,
  "respect_robots_txt": true,
  "notification": {
    "enabled": true,
    "discord_webhook_url": "YOUR_WEBHOOK_URL",
    "telegram_bot_token": "YOUR_BOT_TOKEN",
    "telegram_chat_id": "YOUR_CHAT_ID",
    "price_drop_threshold_percent": 5.0
  }
}
```

## 📸 Screenshots

| Dashboard | Analysis & Chart | Search Engine Comparison |
|---|---|---|
| View the current lowest prices and status of your products via modern cards. | Analyze price volatility, drop percentages, and history on detailed product pages. | Parallel multi-site search that highlights the absolute cheapest alternative for queries like "Playstation 5". |

---
**Developer:** Emir Kılıç  
*This is an open-source project. Feel free to open a Pull Request if you'd like to contribute.*
