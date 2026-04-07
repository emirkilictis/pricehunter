import { Search, Plus, Crosshair } from 'lucide-react';

export default function Header({ onAddClick }) {
  return (
    <header className="header">
      <div className="header-inner">
        <div className="header-brand">
          <div className="header-logo">
            <Crosshair size={20} color="#fff" />
          </div>
          <div>
            <div className="header-title gradient-text">PriceHunter</div>
            <div className="header-subtitle">Distributed Price Monitor</div>
          </div>
        </div>
        <div className="header-actions">
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span className="status-dot online"></span>
            <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Sistem Aktif</span>
          </div>
          <button className="btn btn-primary" onClick={onAddClick} id="add-product-btn">
            <Plus size={16} />
            Ürün Ekle
          </button>
        </div>
      </div>
    </header>
  );
}
