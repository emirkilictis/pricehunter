import { useEffect, useState, useCallback, useMemo } from 'react';
import {
  RefreshCw, ExternalLink, Trash2, ChevronRight,
  ShoppingBag, TrendingDown, TrendingUp, Zap,
  Search, ArrowUpDown, ArrowDown, ArrowUp,
} from 'lucide-react';
import { getProducts, deleteProduct, getPriceStats } from '../api';

export default function ProductList({ refreshKey, onSelect, onProductsLoaded, onRefresh }) {
  const [products, setProducts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [sortBy, setSortBy] = useState('price-desc'); // price-desc, price-asc, name, date
  const [deletingId, setDeletingId] = useState(null);
  const [statsMap, setStatsMap] = useState({});

  const fetchProducts = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await getProducts();
      setProducts(data);
      onProductsLoaded(data);
    } catch (err) {
      setError(err.message);
      setProducts([]);
      onProductsLoaded([]);
    } finally {
      setLoading(false);
    }
  }, [onProductsLoaded]);

  // Fetch stats for all products (non-blocking)
  const fetchStats = useCallback(async (productList) => {
    const map = {};
    await Promise.allSettled(
      productList.map(async (p) => {
        try {
          const s = await getPriceStats(p.id);
          map[p.id] = s;
        } catch { /* ignore */ }
      })
    );
    setStatsMap(map);
  }, []);

  useEffect(() => {
    if (products.length > 0) {
      fetchStats(products);
    }
  }, [products, fetchStats]);

  useEffect(() => {
    fetchProducts();
  }, [fetchProducts, refreshKey]);

  // Auto-refresh every 60 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      fetchProducts();
    }, 60000);
    return () => clearInterval(interval);
  }, [fetchProducts]);

  const handleDelete = async (e, id) => {
    e.stopPropagation();
    if (!confirm('Bu ürünü silmek istediğinize emin misiniz?')) return;
    try {
      setDeletingId(id);
      await deleteProduct(id);
      onRefresh();
    } catch (err) {
      alert('Silme başarısız: ' + err.message);
    } finally {
      setDeletingId(null);
    }
  };

  // Filter and sort products
  const filteredProducts = useMemo(() => {
    let filtered = products;

    // Search filter
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      filtered = filtered.filter(
        (p) =>
          (p.name || '').toLowerCase().includes(q) ||
          (p.site || '').toLowerCase().includes(q) ||
          (p.url || '').toLowerCase().includes(q)
      );
    }

    // Sort
    const sorted = [...filtered];
    switch (sortBy) {
      case 'price-desc':
        sorted.sort((a, b) => (b.last_price || 0) - (a.last_price || 0));
        break;
      case 'price-asc':
        sorted.sort((a, b) => (a.last_price || 0) - (b.last_price || 0));
        break;
      case 'name':
        sorted.sort((a, b) => (a.name || '').localeCompare(b.name || '', 'tr'));
        break;
      case 'date':
        sorted.sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at));
        break;
      default:
        break;
    }

    return sorted;
  }, [products, searchQuery, sortBy]);

  const cycleSortBy = () => {
    const options = ['price-desc', 'price-asc', 'name', 'date'];
    const idx = options.indexOf(sortBy);
    setSortBy(options[(idx + 1) % options.length]);
  };

  const getSortLabel = () => {
    switch (sortBy) {
      case 'price-desc': return 'Fiyat ↓';
      case 'price-asc': return 'Fiyat ↑';
      case 'name': return 'İsim A-Z';
      case 'date': return 'Tarih';
      default: return 'Sırala';
    }
  };

  if (loading) {
    return (
      <div className="loading-container">
        <div className="spinner"></div>
        <div className="loading-text">Ürünler yükleniyor...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="empty-state">
        <div className="empty-icon">⚠️</div>
        <div className="empty-title">Bağlantı Hatası</div>
        <div className="empty-description">
          API sunucusuna bağlanılamadı. Go backend'in çalıştığından emin olun.
          <br />
          <code style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
            go run main.go
          </code>
        </div>
        <button className="btn btn-ghost" onClick={fetchProducts}>
          <RefreshCw size={14} /> Tekrar Dene
        </button>
      </div>
    );
  }

  if (products.length === 0) {
    return (
      <div className="empty-state">
        <div className="empty-icon">🔍</div>
        <div className="empty-title">Henüz ürün eklenmemiş</div>
        <div className="empty-description">
          Fiyat takibi yapmak için bir ürün URL'si ekleyin.
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="product-list-header">
        <div className="product-list-title">
          <ShoppingBag size={18} />
          Takip Edilen Ürünler ({products.length})
        </div>
        <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          <div className="search-box">
            <Search size={14} className="search-icon" />
            <input
              type="text"
              className="search-input"
              placeholder="Ürün ara..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
          <button className="btn btn-ghost btn-sm sort-btn" onClick={cycleSortBy} title="Sıralamayı değiştir">
            <ArrowUpDown size={14} />
            {getSortLabel()}
          </button>
          <button className="btn btn-ghost btn-sm" onClick={onRefresh}>
            <RefreshCw size={14} />
            Yenile
          </button>
        </div>
      </div>

      {filteredProducts.length === 0 && searchQuery && (
        <div className="empty-state" style={{ padding: '40px 0' }}>
          <div className="empty-icon">🔎</div>
          <div className="empty-title">Sonuç bulunamadı</div>
          <div className="empty-description">
            "{searchQuery}" ile eşleşen ürün yok.
          </div>
        </div>
      )}

      <div className="product-grid">
        {filteredProducts.map((product, i) => (
          <div
            key={product.id}
            className={`product-card animate-fade-in-up stagger-${Math.min(i + 1, 5)}${deletingId === product.id ? ' deleting' : ''}`}
            onClick={() => onSelect(product)}
            id={`product-card-${product.id}`}
          >
            <div className="product-thumb-wrapper">
              {product.image_url ? (
                <img
                  src={product.image_url}
                  alt={product.name}
                  className="product-thumb"
                  loading="lazy"
                  onError={(e) => { e.target.style.display = 'none'; e.target.nextSibling.style.display = 'flex'; }}
                />
              ) : null}
              <div className="product-thumb-placeholder" style={product.image_url ? { display: 'none' } : {}}>
                <span style={{ fontSize: '1.5rem' }}>{getSiteIcon(product.site)}</span>
              </div>
            </div>
            <div className="product-info">
              <div className="product-name">{product.name || 'İsimsiz Ürün'}</div>
              <div className="product-meta">
                <span className="product-site">
                  {getSiteIcon(product.site)} {product.site || 'Bilinmeyen'}
                </span>
                <span className="product-url" title={product.url}>
                  {truncateUrl(product.url)}
                </span>
                <span className="product-updated">
                  {formatDate(product.updated_at)}
                </span>
              </div>
            </div>
            <div className="product-price-section">
              {product.last_price > 0 ? (
                <>
                  <div className="product-price has-price">
                    {formatPrice(product.last_price)}
                    <span className="product-currency">{product.currency || 'TL'}</span>
                  </div>
                  {statsMap[product.id]?.is_at_lowest && statsMap[product.id]?.data_points >= 2 && (
                    <div className="lowest-badge-card">
                      <Zap size={10} /> En Düşük
                    </div>
                  )}
                  {statsMap[product.id]?.drop_from_max > 0 && !statsMap[product.id]?.is_at_lowest && (
                    <div className="savings-badge-card">
                      <TrendingDown size={10} /> %{statsMap[product.id].drop_from_max.toFixed(0)} düşüş
                    </div>
                  )}
                </>
              ) : (
                <div className="product-price no-price">Fiyat bekleniyor...</div>
              )}
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '8px', marginTop: '8px' }}>
                <button
                  className="btn btn-danger btn-icon btn-sm"
                  onClick={(e) => handleDelete(e, product.id)}
                  title="Ürünü sil"
                  disabled={deletingId === product.id}
                >
                  {deletingId === product.id ? (
                    <div className="spinner" style={{ width: '14px', height: '14px', borderWidth: '2px' }}></div>
                  ) : (
                    <Trash2 size={14} />
                  )}
                </button>
                <ChevronRight size={18} color="var(--text-muted)" />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function getSiteIcon(site) {
  switch (site) {
    case 'Trendyol': return '🟠';
    case 'Hepsiburada': return '🟣';
    case 'Amazon TR': return '🟡';
    default: return '🌐';
  }
}

function formatPrice(price) {
  return new Intl.NumberFormat('tr-TR', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(price);
}

function truncateUrl(url) {
  if (!url) return '';
  try {
    const u = new URL(url);
    const path = u.pathname.length > 30 ? u.pathname.slice(0, 30) + '...' : u.pathname;
    return u.hostname + path;
  } catch {
    return url.slice(0, 50) + '...';
  }
}

function formatDate(dateStr) {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return '';
  return d.toLocaleDateString('tr-TR', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}
