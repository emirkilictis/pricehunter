import { useMemo } from 'react';
import { Package, TrendingDown, TrendingUp, Clock } from 'lucide-react';

export default function StatsBar({ products }) {
  const stats = useMemo(() => {
    if (!products || products.length === 0) {
      return {
        total: 0,
        avgPrice: 0,
        tracked: 0,
        lastUpdate: '—',
      };
    }

    const withPrice = products.filter((p) => p.last_price > 0);
    const avgPrice = withPrice.length > 0
      ? withPrice.reduce((s, p) => s + p.last_price, 0) / withPrice.length
      : 0;

    let lastUpdate = '—';
    if (products.length > 0) {
      const dates = products
        .map((p) => new Date(p.updated_at))
        .filter((d) => !isNaN(d.getTime()));
      if (dates.length > 0) {
        const latest = new Date(Math.max(...dates));
        lastUpdate = formatTimeAgo(latest);
      }
    }

    return {
      total: products.length,
      avgPrice: avgPrice,
      tracked: withPrice.length,
      lastUpdate,
    };
  }, [products]);

  return (
    <div className="stats-bar">
      <div className="stat-card">
        <div className="stat-icon purple"><Package size={20} color="#a29bfe" /></div>
        <div className="stat-value">{stats.total}</div>
        <div className="stat-label">Toplam Ürün</div>
      </div>
      <div className="stat-card">
        <div className="stat-icon teal"><TrendingDown size={20} color="#00cec9" /></div>
        <div className="stat-value">
          {stats.avgPrice > 0 ? `₺${formatNumber(stats.avgPrice)}` : '—'}
        </div>
        <div className="stat-label">Ortalama Fiyat</div>
      </div>
      <div className="stat-card">
        <div className="stat-icon green"><TrendingUp size={20} color="#00b894" /></div>
        <div className="stat-value">{stats.tracked}</div>
        <div className="stat-label">Fiyatı Takipte</div>
      </div>
      <div className="stat-card">
        <div className="stat-icon orange"><Clock size={20} color="#fdcb6e" /></div>
        <div className="stat-value" style={{ fontSize: '1.1rem' }}>{stats.lastUpdate}</div>
        <div className="stat-label">Son Güncelleme</div>
      </div>
    </div>
  );
}

function formatNumber(num) {
  return new Intl.NumberFormat('tr-TR', {
    maximumFractionDigits: 0,
  }).format(num);
}

function formatTimeAgo(date) {
  const now = new Date();
  const diffMs = now - date;
  const diffMin = Math.floor(diffMs / 60000);

  if (diffMin < 1) return 'Az önce';
  if (diffMin < 60) return `${diffMin} dk önce`;
  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) return `${diffHours} saat önce`;
  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays} gün önce`;
}
