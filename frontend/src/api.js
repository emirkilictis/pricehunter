// API client for PriceHunter backend
const API_BASE = 'http://localhost:8080/api';

async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;
  const config = {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  };

  const response = await fetch(url, config);
  const data = await response.json();

  if (!response.ok || !data.success) {
    throw new Error(data.error || `Request failed: ${response.status}`);
  }

  return data;
}

export async function getProducts() {
  const res = await request('/products');
  return res.data || [];
}

export async function getProduct(id) {
  const res = await request(`/products/${id}`);
  return res.data;
}

export async function getPriceHistory(id, limit = 100) {
  const res = await request(`/products/${id}/history?limit=${limit}`);
  return res.data || [];
}

export async function addProduct(url, name) {
  const res = await request('/products', {
    method: 'POST',
    body: JSON.stringify({ url, name }),
  });
  return res.data;
}

export async function deleteProduct(id) {
  return request(`/products/${id}`, { method: 'DELETE' });
}

export async function healthCheck() {
  const res = await request('/health');
  return res.data;
}

export async function getPriceStats(id) {
  const res = await request(`/products/${id}/stats`);
  return res.data;
}

export async function searchProducts(query) {
  const res = await request(`/search?q=${encodeURIComponent(query)}`);
  return res.data;
}
