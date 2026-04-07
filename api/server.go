// Package api provides a REST API server for the PriceHunter frontend.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"github.com/emirkilic/pricehunter/internal/scraper"
	"github.com/emirkilic/pricehunter/internal/storage"
)

// Server is the REST API server.
type Server struct {
	db      *storage.DB
	engine  *scraper.Engine
	router  *mux.Router
	host    string
	port    int
}

// NewServer creates a new API server.
func NewServer(db *storage.DB, engine *scraper.Engine, host string, port int) *Server {
	s := &Server{
		db:     db,
		engine: engine,
		router: mux.NewRouter(),
		host:   host,
		port:   port,
	}
	s.registerRoutes()
	return s
}

// registerRoutes sets up all API endpoints.
func (s *Server) registerRoutes() {
	api := s.router.PathPrefix("/api").Subrouter()

	api.HandleFunc("/products", s.handleGetProducts).Methods("GET")
	api.HandleFunc("/products", s.handleAddProduct).Methods("POST")
	api.HandleFunc("/products/{id}", s.handleGetProduct).Methods("GET")
	api.HandleFunc("/products/{id}/history", s.handleGetPriceHistory).Methods("GET")
	api.HandleFunc("/products/{id}/stats", s.handleGetPriceStats).Methods("GET")
	api.HandleFunc("/products/{id}", s.handleDeleteProduct).Methods("DELETE")
	api.HandleFunc("/search", s.handleSearch).Methods("GET")
	api.HandleFunc("/health", s.handleHealth).Methods("GET")
}

// Start begins listening for requests.
func (s *Server) Start() error {
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:3000", "http://127.0.0.1:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	handler := c.Handler(s.router)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	log.Printf("[API] Server starting on http://%s", addr)
	return http.ListenAndServe(addr, handler)
}

// --- Response Helpers ---

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Count   int         `json:"count,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Success: false, Error: msg})
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data: map[string]interface{}{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		},
	})
}

func (s *Server) handleGetProducts(w http.ResponseWriter, r *http.Request) {
	products, err := s.db.GetAllProducts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve products")
		log.Printf("[API] GetProducts error: %v", err)
		return
	}

	if products == nil {
		products = []storage.Product{}
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    products,
		Count:   len(products),
	})
}

func (s *Server) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	product, err := s.db.GetProduct(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve product")
		return
	}
	if product == nil {
		writeError(w, http.StatusNotFound, "Product not found")
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    product,
	})
}

func (s *Server) handleGetPriceHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history, err := s.db.GetPriceHistory(id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve price history")
		return
	}

	if history == nil {
		history = []storage.PriceRecord{}
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    history,
		Count:   len(history),
	})
}

type addProductRequest struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

func (s *Server) handleAddProduct(w http.ResponseWriter, r *http.Request) {
	var req addProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "URL is required")
		return
	}

	// Validate URL format
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		writeError(w, http.StatusBadRequest, "URL must start with http:// or https://")
		return
	}

	// Detect site
	site := "Other"
	lower := strings.ToLower(req.URL)
	switch {
	case strings.Contains(lower, "trendyol.com"):
		site = "Trendyol"
	case strings.Contains(lower, "hepsiburada.com"):
		site = "Hepsiburada"
	case strings.Contains(lower, "amazon.com.tr"):
		site = "Amazon TR"
	}

	if req.Name == "" {
		req.Name = "Unnamed Product"
	}

	id, err := s.db.AddProduct(req.URL, req.Name, site)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to add product")
		log.Printf("[API] AddProduct error: %v", err)
		return
	}

	product, _ := s.db.GetProduct(id)

	writeJSON(w, http.StatusCreated, apiResponse{
		Success: true,
		Data:    product,
	})
}

func (s *Server) handleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	if err := s.db.DeleteProduct(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete product")
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    map[string]string{"message": "Product deleted"},
	})
}

func (s *Server) handleGetPriceStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	stats, err := s.db.GetPriceStats(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve price stats")
		log.Printf("[API] GetPriceStats error: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    stats,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "Missing search query (q)")
		return
	}
	if len(query) < 2 {
		writeError(w, http.StatusBadRequest, "Query too short")
		return
	}

	log.Printf("[API] Search request: %q", query)
	results := s.engine.SearchAcrossSites(r.Context(), query)

	// Sort: successful results first, then by price ascending
	successful := make([]interface{}, 0)
	failed := make([]interface{}, 0)

	// Build sortable list
	type sortableResult struct {
		Site     string  `json:"site"`
		Name     string  `json:"name"`
		Price    float64 `json:"price"`
		Currency string  `json:"currency"`
		URL      string  `json:"url"`
		ImageURL string  `json:"image_url"`
		Error    string  `json:"error,omitempty"`
	}

	var sorted []sortableResult
	for _, r := range results {
		sorted = append(sorted, sortableResult{
			Site:     r.Site,
			Name:     r.Name,
			Price:    r.Price,
			Currency: r.Currency,
			URL:      r.URL,
			ImageURL: r.ImageURL,
			Error:    r.Error,
		})
	}

	// Sort by price ascending (0 = no price, goes last)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			pi, pj := sorted[i].Price, sorted[j].Price
			if pi == 0 || (pj > 0 && pj < pi) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// separate successful from failed
	for _, r := range sorted {
		if r.Error != "" {
			failed = append(failed, r)
		} else {
			successful = append(successful, r)
		}
	}

	allResults := append(successful, failed...)
	_ = allResults

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    sorted,
	})
}
