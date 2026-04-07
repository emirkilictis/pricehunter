import { useEffect, useState } from 'react';
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid,
  Tooltip, ResponsiveContainer, Area, AreaChart,
  ReferenceLine,
} from 'recharts';
import {
  ArrowLeft, TrendingDown, TrendingUp, Calendar, ExternalLink,
  Zap, Target, BarChart3, Clock, AlertTriangle, CheckCircle2,
} from 'lucide-react';
import { getPriceHistory, getPriceStats } from '../api';

export default function PriceChart({ product, onBack }) {
  const [history, setHistory] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function loadData() {
      try {
        setLoading(true);
        const [historyData, statsData] = await Promise.all([
          getPriceHistory(product.id),
          getPriceStats(product.id),
        ]);
        setHistory(historyData);
        setStats(statsData);
      } catch (err) {
        console.error('Failed to load data:', err);
        setHistory([]);
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, [product.id]);

  const chartData = history.map((r) => ({
    date: new Date(r.scraped_at).toLocaleDateString('tr-TR', {
      day: '2-digit',
      month: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    }),
    fullDate: new Date(r.scraped_at).toLocaleString('tr-TR'),
    price: r.price,
  }));

  const prices = history.map((r) => r.price).filter((p) => p > 0);
  const minPrice = stats?.min_price || (prices.length > 0 ? Math.min(...prices) : 0);
  const maxPrice = stats?.max_price || (prices.length > 0 ? Math.max(...prices) : 0);
  const avgPrice = stats?.avg_price || (prices.length > 0 ? prices.reduce((a, b) => a + b, 0) / prices.length : 0);
  const currentPrice = product.last_price || 0;

  // Price position gauge (0% = at lowest, 100% = at highest)
  const priceRange = maxPrice - minPrice;
  const pricePosition = priceRange > 0
    ? ((currentPrice - minPrice) / priceRange) * 100
    : 50;

  const changeFromFirst = prices.length >= 2
    ? ((prices[prices.length - 1] - prices[0]) / prices[0]) * 100
    : 0;

  // Smart insight
  const getInsight = () => {
    if (!stats || stats.data_points < 2) return null;
    if (stats.is_at_lowest) {
      return {
        type: 'success',
        icon: <CheckCircle2 size={16} />,
        title: '🎉 En Düşük Fiyatta!',
        text: `Bu ürün şu an takip ettiğimiz süre boyunca gördüğü en düşük fiyatta. Satın almak için ideal bir zaman!`,
      };
    }
    if (stats.drop_from_max > 10) {
      return {
        type: 'good',
        icon: <TrendingDown size={16} />,
        title: '📉 Fiyat Düşmüş!',
        text: `En yüksek fiyatından %${stats.drop_from_max.toFixed(1)} daha düşük. ₺${formatPrice(maxPrice - currentPrice)} tasarruf!`,
      };
    }
    if (stats.is_at_highest) {
      return {
        type: 'warning',
        icon: <AlertTriangle size={16} />,
        title: '⚠️ En Yüksek Fiyatta',
        text: `Bu ürün şu an gördüğü en yüksek fiyatta. Beklemek isteyebilirsiniz.`,
      };
    }
    if (currentPrice < avgPrice) {
      return {
        type: 'good',
        icon: <Target size={16} />,
        title: '💡 Ortalamanın Altında',
        text: `Mevcut fiyat ortalama fiyattan ₺${formatPrice(avgPrice - currentPrice)} daha düşük.`,
      };
    }
    return {
      type: 'neutral',
      icon: <BarChart3 size={16} />,
      title: 'ℹ️ Normal Seviyede',
      text: `Fiyat ortalama civarında seyrediyor.`,
    };
  };

  const insight = getInsight();

  return (
    <div className="chart-view">
      <div className="chart-header">
        <div>
          <button className="btn btn-ghost btn-sm" onClick={onBack} style={{ marginBottom: '12px' }}>
            <ArrowLeft size={14} />
            Geri Dön
          </button>
          <div className="chart-product-info" style={{ display: 'flex', gap: '20px', alignItems: 'flex-start' }}>
            {product.image_url ? (
              <div className="detail-product-image">
                <img
                  src={product.image_url}
                  alt={product.name}
                  style={{
                    width: '120px',
                    height: '120px',
                    objectFit: 'cover',
                    borderRadius: 'var(--radius-lg)',
                    border: '1px solid var(--border-subtle)',
                    flexShrink: 0,
                  }}
                  onError={(e) => { e.target.style.display = 'none'; }}
                />
              </div>
            ) : null}
            <div>
              <h2>{product.name || 'İsimsiz Ürün'}</h2>
              <div style={{ display: 'flex', gap: '8px', marginTop: '8px', flexWrap: 'wrap', alignItems: 'center' }}>
                <span className="chart-site-badge">{product.site}</span>
                <a
                  href={product.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  style={{ fontSize: '0.8rem', display: 'flex', alignItems: 'center', gap: '4px' }}
                >
                  <ExternalLink size={12} /> Siteye Git
                </a>
              </div>
            </div>
          </div>
        </div>
        <div style={{ textAlign: 'right' }}>
          {currentPrice > 0 && (
            <>
              <div style={{ fontSize: '2rem', fontWeight: 800, fontFamily: 'var(--font-mono)', letterSpacing: '-0.03em' }}>
                ₺{formatPrice(currentPrice)}
              </div>
              {stats?.is_at_lowest && (
                <div className="lowest-price-badge" style={{
                  display: 'inline-flex', alignItems: 'center', gap: '4px',
                  background: 'linear-gradient(135deg, #00b89420, #00b89410)',
                  border: '1px solid #00b89450',
                  color: '#00b894',
                  padding: '4px 12px', borderRadius: '20px',
                  fontSize: '0.75rem', fontWeight: 700, marginTop: '4px',
                }}>
                  <Zap size={12} /> EN DÜŞÜK FİYAT
                </div>
              )}
              {changeFromFirst !== 0 && (
                <div className={`price-change ${changeFromFirst > 0 ? 'up' : 'down'}`} style={{ marginTop: '4px' }}>
                  {changeFromFirst > 0 ? <TrendingUp size={12} /> : <TrendingDown size={12} />}
                  {changeFromFirst > 0 ? '+' : ''}{changeFromFirst.toFixed(1)}%
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {/* Smart Insight Banner */}
      {insight && (
        <div className={`insight-banner insight-${insight.type}`}>
          <div className="insight-icon">{insight.icon}</div>
          <div>
            <div className="insight-title">{insight.title}</div>
            <div className="insight-text">{insight.text}</div>
          </div>
        </div>
      )}

      {/* Price Analysis Panel */}
      {currentPrice > 0 && stats && (
        <div className="price-analysis-panel">
          {/* Price Position Gauge */}
          <div className="price-gauge-section">
            <div className="gauge-header">
              <span className="gauge-title"><Target size={14} /> Fiyat Pozisyonu</span>
              <span className="gauge-value" style={{
                color: pricePosition < 30 ? '#00b894' : pricePosition > 70 ? '#e17055' : '#fdcb6e',
              }}>
                {pricePosition < 30 ? 'Düşük Seviye' : pricePosition > 70 ? 'Yüksek Seviye' : 'Orta Seviye'}
              </span>
            </div>
            <div className="price-gauge">
              <div className="gauge-track">
                <div
                  className="gauge-fill"
                  style={{
                    width: `${Math.min(100, Math.max(0, pricePosition))}%`,
                    background: pricePosition < 30
                      ? 'linear-gradient(90deg, #00b894, #55efc4)'
                      : pricePosition > 70
                        ? 'linear-gradient(90deg, #fdcb6e, #e17055)'
                        : 'linear-gradient(90deg, #00cec9, #fdcb6e)',
                  }}
                />
                <div className="gauge-marker" style={{ left: `${Math.min(97, Math.max(3, pricePosition))}%` }} />
              </div>
              <div className="gauge-labels">
                <span>₺{formatCompact(minPrice)}</span>
                <span>₺{formatCompact(avgPrice)}</span>
                <span>₺{formatCompact(maxPrice)}</span>
              </div>
            </div>
          </div>

          {/* Stats Cards */}
          <div className="price-stats-grid">
            <div className="mini-stat-card lowest">
              <div className="mini-stat-icon"><TrendingDown size={16} /></div>
              <div className="mini-stat-info">
                <div className="mini-stat-value">₺{formatPrice(minPrice)}</div>
                <div className="mini-stat-label">En Düşük</div>
              </div>
              {stats.is_at_lowest && <div className="mini-stat-badge">ŞU AN</div>}
            </div>
            <div className="mini-stat-card average">
              <div className="mini-stat-icon"><BarChart3 size={16} /></div>
              <div className="mini-stat-info">
                <div className="mini-stat-value">₺{formatPrice(avgPrice)}</div>
                <div className="mini-stat-label">Ortalama</div>
              </div>
            </div>
            <div className="mini-stat-card highest">
              <div className="mini-stat-icon"><TrendingUp size={16} /></div>
              <div className="mini-stat-info">
                <div className="mini-stat-value">₺{formatPrice(maxPrice)}</div>
                <div className="mini-stat-label">En Yüksek</div>
              </div>
              {stats.is_at_highest && <div className="mini-stat-badge warning">ŞU AN</div>}
            </div>
            <div className="mini-stat-card savings">
              <div className="mini-stat-icon"><Zap size={16} /></div>
              <div className="mini-stat-info">
                <div className="mini-stat-value">
                  {stats.drop_from_max > 0 ? `%${stats.drop_from_max.toFixed(1)}` : '—'}
                </div>
                <div className="mini-stat-label">Max'tan Düşüş</div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Chart */}
      <div className="chart-container">
        <h3>📊 Fiyat Geçmişi Grafiği</h3>
        {loading ? (
          <div className="loading-container">
            <div className="spinner"></div>
            <div className="loading-text">Fiyat geçmişi yükleniyor...</div>
          </div>
        ) : chartData.length === 0 ? (
          <div className="chart-empty">
            <div className="chart-empty-icon">📉</div>
            <div>Henüz fiyat verisi bulunmuyor.</div>
            <div style={{ color: 'var(--text-muted)', fontSize: '0.85rem', marginTop: '8px' }}>
              İlk scrape tamamlandıktan sonra grafik burada görünecek.
            </div>
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={350}>
            <AreaChart data={chartData} margin={{ top: 10, right: 10, bottom: 0, left: 10 }}>
              <defs>
                <linearGradient id="priceGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#6c5ce7" stopOpacity={0.3} />
                  <stop offset="50%" stopColor="#a29bfe" stopOpacity={0.1} />
                  <stop offset="100%" stopColor="#00cec9" stopOpacity={0.02} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border-subtle)" />
              <XAxis
                dataKey="date"
                stroke="var(--text-muted)"
                fontSize={11}
                tickLine={false}
                axisLine={{ stroke: 'var(--border-subtle)' }}
              />
              <YAxis
                stroke="var(--text-muted)"
                fontSize={11}
                tickLine={false}
                axisLine={{ stroke: 'var(--border-subtle)' }}
                tickFormatter={(v) => `₺${formatCompact(v)}`}
                domain={[
                  (dataMin) => Math.floor(dataMin * 0.98),
                  (dataMax) => Math.ceil(dataMax * 1.02),
                ]}
              />
              <Tooltip content={<CustomTooltip />} />
              {/* Min price reference */}
              {minPrice > 0 && (
                <ReferenceLine
                  y={minPrice}
                  stroke="#00b894"
                  strokeDasharray="3 3"
                  strokeOpacity={0.6}
                  label={{
                    value: 'Min',
                    position: 'left',
                    fill: '#00b894',
                    fontSize: 10,
                  }}
                />
              )}
              {/* Average reference */}
              {avgPrice > 0 && (
                <ReferenceLine
                  y={avgPrice}
                  stroke="var(--accent-secondary)"
                  strokeDasharray="5 5"
                  strokeOpacity={0.4}
                  label={{
                    value: 'Ort.',
                    position: 'right',
                    fill: 'var(--text-muted)',
                    fontSize: 10,
                  }}
                />
              )}
              {/* Max price reference */}
              {maxPrice > 0 && maxPrice !== minPrice && (
                <ReferenceLine
                  y={maxPrice}
                  stroke="#e17055"
                  strokeDasharray="3 3"
                  strokeOpacity={0.6}
                  label={{
                    value: 'Max',
                    position: 'left',
                    fill: '#e17055',
                    fontSize: 10,
                  }}
                />
              )}
              <Area
                type="monotone"
                dataKey="price"
                stroke="#6c5ce7"
                strokeWidth={2.5}
                fill="url(#priceGradient)"
                dot={{ r: 3, fill: '#6c5ce7', strokeWidth: 0 }}
                activeDot={{ r: 6, fill: '#a29bfe', strokeWidth: 2, stroke: '#fff' }}
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* History Table */}
      {history.length > 0 && (
        <div className="history-table-wrapper">
          <h3>📋 Fiyat Geçmişi Tablosu</h3>
          <table className="history-table">
            <thead>
              <tr>
                <th>#</th>
                <th>Fiyat</th>
                <th>Durum</th>
                <th>Değişim</th>
                <th>Tarih</th>
              </tr>
            </thead>
            <tbody>
              {[...history].reverse().map((record, i) => {
                const prevRecord = history[history.length - 1 - i - 1];
                const change = prevRecord
                  ? ((record.price - prevRecord.price) / prevRecord.price) * 100
                  : 0;
                const isMin = record.price === minPrice;
                const isMax = record.price === maxPrice;
                return (
                  <tr key={record.id} className={isMin ? 'row-lowest' : isMax ? 'row-highest' : ''}>
                    <td style={{ color: 'var(--text-muted)' }}>{history.length - i}</td>
                    <td className="price-cell">
                      ₺{formatPrice(record.price)}
                      {isMin && <span className="price-badge-mini green">MIN</span>}
                      {isMax && <span className="price-badge-mini red">MAX</span>}
                    </td>
                    <td>
                      {isMin ? (
                        <span style={{ color: '#00b894', fontSize: '0.75rem', fontWeight: 600 }}>🟢 En Düşük</span>
                      ) : isMax ? (
                        <span style={{ color: '#e17055', fontSize: '0.75rem', fontWeight: 600 }}>🔴 En Yüksek</span>
                      ) : (
                        <span style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>—</span>
                      )}
                    </td>
                    <td>
                      {change !== 0 ? (
                        <span className={`price-change ${change > 0 ? 'up' : 'down'}`}>
                          {change > 0 ? '+' : ''}{change.toFixed(1)}%
                        </span>
                      ) : (
                        <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>—</span>
                      )}
                    </td>
                    <td className="date-cell">
                      {new Date(record.scraped_at).toLocaleString('tr-TR')}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function CustomTooltip({ active, payload }) {
  if (!active || !payload || !payload.length) return null;
  const data = payload[0].payload;
  return (
    <div style={{
      background: 'var(--bg-elevated)',
      border: '1px solid var(--border-primary)',
      borderRadius: 'var(--radius-md)',
      padding: '12px 16px',
      boxShadow: 'var(--shadow-lg)',
    }}>
      <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: '4px' }}>
        {data.fullDate}
      </div>
      <div style={{ fontSize: '1.1rem', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>
        ₺{formatPrice(data.price)}
      </div>
    </div>
  );
}

function formatPrice(price) {
  return new Intl.NumberFormat('tr-TR', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(price);
}

function formatCompact(num) {
  if (num >= 1000) return (num / 1000).toFixed(0) + 'K';
  return num.toFixed(0);
}
