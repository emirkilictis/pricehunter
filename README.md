# 🎯 PriceHunter

PriceHunter, e-ticaret sitelerindeki (Trendyol, Hepsiburada, Amazon TR vb.) ürünlerin fiyatlarını otomatik olarak takip eden, analiz eden ve fiyatı düştüğünde bildirim gönderen **dağıtık mimariye sahip gelişmiş bir fiyat takip platformudur.** 

Backend tarafında yüksek performanslı Go, frontend tarafında modern React kullanılarak uçtan uca eksiksiz bir ürün olarak tasarlanmıştır. Güçlü scraping (veri çekme) motoru sayesinde WAF ve anti-bot engellerini (Cloudflare, Akamai vb.) bypass edecek yeteneklerle donatılmıştır.

## ✨ Öne Çıkan Özellikler

* **🔎 Akıllı Ürün Arama ve Karşılaştırma:** Tek bir ürün adıyla birden fazla sitede eşzamanlı arama yapar ve en ucuz fiyatlısını saptayarak sana sunar.
* **🛡️ Gelişmiş Anti-Bot Scraping:** TLS fingerprinting (`uTLS`), proxy rotasyonu, User-Agent havuzu ve gerektiğinde devreye giren **Headless Chrome (Chromedp)** ile sitelerden asla banlanmaz.
* **📈 Detaylı İstatistikler & Grafikler:** Ürünlerin daha önceki en yüksek, en düşük ve ortalama fiyatlarını analiz eder; fiyatın şu an hangi pozisyonda olduğunu (Gauge) ve fiyat geçmişini interaktif grafiklerle (`recharts`) gösterir.
* **⚡ Asenkron Worker Pool:** Arka planda `goroutine`'ler ile tamamen kilitlenmesiz bir şekilde, belirlediğin periyotlarda (örn: 30 dakikada bir) tüm bağlantıları eşzamanlı olarak tarar.
* **🔔 Telegram ve Discord Entegrasyonu:** Fiyat belirlediğin yüzde (%5 vb.) oranında düştüğünde webhook üzerinden anında cihazlarına bildirim gönderir.

## 🛠️ Kullanılan Teknolojiler

**Backend Motoru:**
* **Dil:** Golang (Go)
* **Veritabanı:** SQLite (Maksimum okuma/yazma performansı için `WAL` modunda)
* **API:** Gorilla Mux (REST JSON API)
* **Scraping Araçları:** goquery, chromedp, utls

**Frontend Dashboard:**
* **Altyapı:** React (Vite)
* **Stil:** Modern Saf CSS (Glassmorphism, Pulse animasyonları, Responsive tasarım)
* **Grafikler:** Recharts
* **İkonlar:** Lucide React

## 🚀 Kurulum (Local Development)

Projeyi kendi bilgisayarında çalıştırmak için Go ve Node.js gerekmektedir.

### 1️⃣ Repoyu İndirin
```bash
git clone https://github.com/emirkilictis/pricehunter.git
cd pricehunter
```

### 2️⃣ Backend'i Başlatın (Go)
```bash
# Bağımlılıkları indirin
go mod tidy 

# Projeyi derleyin ve çalıştırın
go build -o pricehunter .
./pricehunter
```
*Backend `http://localhost:8080` üzerinde API yayınına başlayacaktır.*

### 3️⃣ Frontend'i Başlatın (React)
Yeni bir terminal sekmesi açın ve şu komutları girin:
```bash
cd frontend
npm install
npm run dev
```
*Panel `http://localhost:5173` üzerinde açılacaktır.*

## ⚙️ Yapılandırma (Config)

Veri çekme ayarları, bildirimler ve proxy kurulumu için ana dizindeki `config.json` dosyasını düzenleyebilirsiniz.

```json
{
  "scrape_interval_minutes": 30,
  "max_workers": 5,
  "respect_robots_txt": true,
  "notification": {
    "enabled": true,
    "discord_webhook_url": "SENIN_WEBHOOK_URL",
    "telegram_bot_token": "SENIN_BOT_TOKEN",
    "telegram_chat_id": "SENIN_CHAT_ID",
    "price_drop_threshold_percent": 5.0
  }
}
```

## 📸 Ekran Görüntüleri

| Dashboard | Analiz & Grafik | Arama Motoru Karşılaştırması |
|---|---|---|
| Modern kart mimarisi üzerinden ürünlerin o anki en ucuz fiyat durumlarını izleyebilirsiniz. | Ürünün fiyat dalgalanmasını ve "şu an ne kadar ucuz" olduğunu detay sayfasından analiz edebilirsiniz. | "Playstation 5" tarzı bir aramada paralel siteleri döner ve en düşük fiyatı vurgular. |

---
**Geliştirici:** Emir Kılıç  
*Açık kaynaklı bir projedir. Katkı sağlamak isteyenler Pull Request gönderebilirler.*
