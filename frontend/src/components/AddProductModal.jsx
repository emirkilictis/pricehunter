import { useState, useRef, useEffect } from 'react';
import { Search, Plus, X, Link, Tag, Zap, TrendingDown, ExternalLink, Loader2 } from 'lucide-react';
import { addProduct, searchProducts } from '../api';

export default function AddProductModal({ onClose, onAdded }) {
  const [mode, setMode] = useState('search'); // 'search' | 'url'

  // Search mode state
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [searchResults, setSearchResults] = useState(null);
  const [searchError, setSearchError] = useState(null);
  const [addingUrl, setAddingUrl] = useState(null);

  // URL mode state
  const [url, setUrl] = useState('');
  const [urlName, setUrlName] = useState('');
  const [urlLoading, setUrlLoading] = useState(false);
  const [urlError, setUrlError] = useState(null);

  const searchInputRef = useRef(null);

  useEffect(() => {
    if (mode === 'search' && searchInputRef.current) {
      searchInputRef.current.focus();
    }
  }, [mode]);

  const handleSearch = async (e) => {
    e.preventDefault();
    if (!query.trim() || query.trim().length < 2) {
      setSearchError('En az 2 karakter girin.');
      return;
    }
    try {
      setSearching(true);
      setSearchError(null);
      setSearchResults(null);
      const data = await searchProducts(query.trim());
      setSearchResults(data || []);
    } catch (err) {
      setSearchError(err.message || 'Arama sırasında bir hata oluştu.');
    } finally {
      setSearching(false);
    }
  };

  const handleTrack = async (result) => {
    try {
      setAddingUrl(result.url);
      await addProduct(result.url, result.name);
      onAdded();
    } catch (err) {
      setSearchError('Eklenemedi: ' + err.message);
      setAddingUrl(null);
    }
  };

  const handleUrlSubmit = async (e) => {
    e.preventDefault();
    if (!url.trim()) { setUrlError('Lütfen bir URL girin.'); return; }
    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      setUrlError('URL http:// veya https:// ile başlamalıdır.');
      return;
    }
    try {
      setUrlLoading(true);
      setUrlError(null);
      await addProduct(url.trim(), urlName.trim() || undefined);
      onAdded();
    } catch (err) {
      setUrlError(err.message || 'Ürün eklenirken bir hata oluştu.');
    } finally {
      setUrlLoading(false);
    }
  };

  const handleOverlayClick = (e) => {
    if (e.target === e.currentTarget) onClose();
  };

  const getBestPrice = (results) => {
    const valid = (results || []).filter(r => r.price > 0 && !r.error);
    if (!valid.length) return null;
    return valid.reduce((a, b) => a.price < b.price ? a : b);
  };

  const best = getBestPrice(searchResults);

  return (
    <div className="modal-overlay" onClick={handleOverlayClick}>
      <div className="modal modal-wide animate-fade-in-up" role="dialog" aria-modal="true" id="add-product-modal">
        {/* Header */}
        <div className="modal-title">
          <Plus size={20} color="var(--accent-primary-light)" />
          Ürün Ekle
        </div>

        {/* Tabs */}
        <div className="modal-tabs">
          <button
            className={`modal-tab ${mode === 'search' ? 'active' : ''}`}
            onClick={() => setMode('search')}
          >
            <Search size={14} /> Ürün Ara
          </button>
          <button
            className={`modal-tab ${mode === 'url' ? 'active' : ''}`}
            onClick={() => setMode('url')}
          >
            <Link size={14} /> URL ile Ekle
          </button>
        </div>

        {/* Search Mode */}
        {mode === 'search' && (
          <div>
            <form onSubmit={handleSearch} style={{ marginBottom: '16px' }}>
              <div style={{ display: 'flex', gap: '8px' }}>
                <div style={{ position: 'relative', flex: 1 }}>
                  <Search size={16} style={{ position: 'absolute', left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
                  <input
                    ref={searchInputRef}
                    className="form-input"
                    style={{ paddingLeft: '38px' }}
                    placeholder='Ürün adı girin... (ör: iPhone 16, Samsung S24)'
                    value={query}
                    onChange={e => { setQuery(e.target.value); setSearchError(null); }}
                  />
                </div>
                <button type="submit" className="btn btn-primary" disabled={searching}>
                  {searching ? <Loader2 size={16} className="spin" /> : <Search size={16} />}
                  {searching ? 'Aranıyor...' : 'Ara'}
                </button>
              </div>
              <div className="form-hint" style={{ marginTop: '6px' }}>
                Trendyol, Hepsiburada ve Amazon TR'de eşzamanlı arama yapılır
              </div>
            </form>

            {searchError && <div className="form-error">{searchError}</div>}

            {searching && (
              <div className="search-loading">
                <div className="search-loading-sites">
                  {['Trendyol', 'Hepsiburada', 'Amazon TR'].map(site => (
                    <div key={site} className="search-loading-site">
                      <div className="spinner" style={{ width: '14px', height: '14px', borderWidth: '2px' }} />
                      {getSiteIcon(site)} {site}'de aranıyor...
                    </div>
                  ))}
                </div>
              </div>
            )}

            {searchResults !== null && !searching && (
              <div>
                {searchResults.length === 0 ? (
                  <div className="search-no-results">Sonuç bulunamadı. Farklı bir arama deneyin.</div>
                ) : (
                  <div className="search-results-list">
                    <div className="search-results-header">
                      🔍 {searchResults.filter(r => !r.error).length} sitede sonuç bulundu
                      {best && (
                        <span className="search-best-label">
                          <Zap size={12} /> En Ucuz: {getSiteIcon(best.site)} {best.site}
                        </span>
                      )}
                    </div>
                    {searchResults.map((result, i) => (
                      <div
                        key={i}
                        className={`search-result-card ${result === best ? 'best-price' : ''} ${result.error ? 'has-error' : ''}`}
                      >
                        {result === best && (
                          <div className="best-price-ribbon"><Zap size={10} /> En Ucuz</div>
                        )}
                        <div className="search-result-left">
                          {result.image_url ? (
                            <img src={result.image_url} alt={result.name} className="search-result-img" onError={e => e.target.style.display='none'} />
                          ) : (
                            <div className="search-result-img-placeholder">{getSiteIcon(result.site)}</div>
                          )}
                        </div>
                        <div className="search-result-mid">
                          <div className="search-result-site">
                            {getSiteIcon(result.site)} {result.site}
                          </div>
                          <div className="search-result-name">
                            {result.error ? (
                              <span style={{ color: 'var(--text-muted)', fontStyle: 'italic' }}>Sonuç alınamadı</span>
                            ) : (
                              result.name || 'Ürün Adı Yok'
                            )}
                          </div>
                          {result.url && !result.error && (
                            <a href={result.url} target="_blank" rel="noreferrer" className="search-result-link" onClick={e => e.stopPropagation()}>
                              <ExternalLink size={11} /> Ürünü Gör
                            </a>
                          )}
                        </div>
                        <div className="search-result-right">
                          {result.price > 0 && !result.error ? (
                            <>
                              <div className="search-result-price">
                                {formatPrice(result.price)}
                                <span className="search-result-currency"> TL</span>
                              </div>
                              {best && result !== best && result.price > 0 && (
                                <div className="search-result-diff">
                                  <TrendingDown size={10} />
                                  +{formatPrice(result.price - best.price)} fazla
                                </div>
                              )}
                              <button
                                className="btn btn-primary btn-sm"
                                style={{ marginTop: '8px', width: '100%' }}
                                onClick={() => handleTrack(result)}
                                disabled={!!addingUrl}
                              >
                                {addingUrl === result.url ? (
                                  <><div className="spinner" style={{ width: '12px', height: '12px', borderWidth: '2px' }} /> Ekleniyor</>
                                ) : (
                                  <><Plus size={12} /> Takip Et</>
                                )}
                              </button>
                            </>
                          ) : (
                            <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>—</div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* URL Mode */}
        {mode === 'url' && (
          <form onSubmit={handleUrlSubmit}>
            {urlError && <div className="form-error">{urlError}</div>}
            <div className="form-group">
              <label className="form-label" htmlFor="product-url">
                <Link size={12} style={{ verticalAlign: 'middle', marginRight: '4px' }} />
                Ürün URL'si
              </label>
              <input
                id="product-url"
                className="form-input"
                type="url"
                placeholder="https://www.trendyol.com/urun/..."
                value={url}
                onChange={e => setUrl(e.target.value)}
                autoFocus
                required
              />
              <div className="form-hint">Trendyol, Hepsiburada veya Amazon.com.tr ürün URL'si</div>
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="product-name">
                <Tag size={12} style={{ verticalAlign: 'middle', marginRight: '4px' }} />
                Ürün Adı (opsiyonel)
              </label>
              <input
                id="product-name"
                className="form-input"
                type="text"
                placeholder="Otomatik algılanır..."
                value={urlName}
                onChange={e => setUrlName(e.target.value)}
              />
            </div>
            <div className="modal-actions">
              <button type="button" className="btn btn-ghost" onClick={onClose} disabled={urlLoading}>
                <X size={14} /> İptal
              </button>
              <button type="submit" className="btn btn-primary" disabled={urlLoading} id="submit-product-btn">
                {urlLoading ? (
                  <><div className="spinner" style={{ width: '14px', height: '14px', borderWidth: '2px' }} /> Ekleniyor...</>
                ) : (
                  <><Plus size={14} /> Ürünü Ekle</>
                )}
              </button>
            </div>
          </form>
        )}

        {/* Close button */}
        <button className="modal-close-btn" onClick={onClose} aria-label="Kapat">
          <X size={18} />
        </button>
      </div>
    </div>
  );
}

function getSiteIcon(site) {
  if (site?.includes('Trendyol')) return '🟠';
  if (site?.includes('Hepsiburada')) return '🟣';
  if (site?.includes('Amazon')) return '🟡';
  return '🌐';
}

function formatPrice(price) {
  return new Intl.NumberFormat('tr-TR', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(price);
}
