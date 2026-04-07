import { useState, useCallback } from 'react';
import './App.css';
import Header from './components/Header';
import StatsBar from './components/StatsBar';
import ProductList from './components/ProductList';
import PriceChart from './components/PriceChart';
import AddProductModal from './components/AddProductModal';

function App() {
  const [products, setProducts] = useState([]);
  const [selectedProduct, setSelectedProduct] = useState(null);
  const [showAddModal, setShowAddModal] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);

  const handleRefresh = useCallback(() => {
    setRefreshKey((k) => k + 1);
  }, []);

  const handleProductSelect = useCallback((product) => {
    setSelectedProduct(product);
  }, []);

  const handleBackToList = useCallback(() => {
    setSelectedProduct(null);
  }, []);

  const handleProductAdded = useCallback(() => {
    setShowAddModal(false);
    handleRefresh();
  }, [handleRefresh]);

  return (
    <div className="app">
      <Header onAddClick={() => setShowAddModal(true)} />
      <main className="main-content">
        <StatsBar products={products} />
        {selectedProduct ? (
          <PriceChart product={selectedProduct} onBack={handleBackToList} />
        ) : (
          <ProductList
            refreshKey={refreshKey}
            onSelect={handleProductSelect}
            onProductsLoaded={setProducts}
            onRefresh={handleRefresh}
          />
        )}
      </main>
      {showAddModal && (
        <AddProductModal
          onClose={() => setShowAddModal(false)}
          onAdded={handleProductAdded}
        />
      )}
    </div>
  );
}

export default App;
