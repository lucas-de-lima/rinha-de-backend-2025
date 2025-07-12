package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
)

var (
	// Atomic counters for metrics
	requestCount int64
	successCount int64
	errorCount   int64

	// BRUTO Cache for summary data
	brutoCache = &BRUTOCache{
		data: make(map[string]interface{}),
		mu:   sync.RWMutex{},
	}

	// BRUTO Summary Response
	brutoSummary = &BRUTOSummary{
		Default:  ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		Fallback: ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		mu:       sync.RWMutex{},
	}
)

// BRUTO Cache
type BRUTOCache struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

func (c *BRUTOCache) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[key]
}

func (c *BRUTOCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

// BRUTO Summary Response
type HTTPSummaryResponse struct {
	Default  ProcessorSummary `json:"default"`
	Fallback ProcessorSummary `json:"fallback"`
}

type ProcessorSummary struct {
	TotalRequests int     `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

// BRUTO Summary with thread-safe updates
type BRUTOSummary struct {
	Default  ProcessorSummary
	Fallback ProcessorSummary
	mu       sync.RWMutex
}

func (s *BRUTOSummary) UpdateDefault(requests int, amount float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Default.TotalRequests += requests
	s.Default.TotalAmount += amount
}

func (s *BRUTOSummary) UpdateFallback(requests int, amount float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Fallback.TotalRequests += requests
	s.Fallback.TotalAmount += amount
}

func (s *BRUTOSummary) GetSummary() HTTPSummaryResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return HTTPSummaryResponse{
		Default:  s.Default,
		Fallback: s.Fallback,
	}
}

func main() {
	// Create router
	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}).Methods("GET")

	// Summary endpoint
	router.HandleFunc("/summary", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		handleSummary(w, r)
	}).Methods("GET")

	// Payments summary endpoint for k6
	router.HandleFunc("/payments-summary", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		handlePaymentsSummary(w, r)
	}).Methods("GET")

	// Start server with optimized settings
	server := &http.Server{
		Addr:         ":8445",
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Println("Summary Service BRUTO starting on :8445")
	log.Fatal(server.ListenAndServe())
}

// BRUTO: Handle summary with aggressive caching
func handleSummary(w http.ResponseWriter, r *http.Request) {
	// BRUTO: Check cache first
	if cached := brutoCache.Get("summary"); cached != nil {
		if summary, ok := cached.(HTTPSummaryResponse); ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(summary)
			atomic.AddInt64(&successCount, 1)
			return
		}
	}

	// BRUTO: Get fresh summary
	summary := brutoSummary.GetSummary()

	// BRUTO: Cache for 1 second
	brutoCache.Set("summary", summary)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
	atomic.AddInt64(&successCount, 1)
}

// BRUTO: Handle payments summary with query parameters
func handlePaymentsSummary(w http.ResponseWriter, r *http.Request) {
	// BRUTO: Check cache first
	cacheKey := fmt.Sprintf("summary_%s_%s", r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if cached := brutoCache.Get(cacheKey); cached != nil {
		if summary, ok := cached.(HTTPSummaryResponse); ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(summary)
			atomic.AddInt64(&successCount, 1)
			return
		}
	}

	// BRUTO: Get fresh summary
	summary := brutoSummary.GetSummary()

	// BRUTO: Cache for 1 second
	brutoCache.Set(cacheKey, summary)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
	atomic.AddInt64(&successCount, 1)
}
