package main

import (
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

	// BRUTO Cache for summary data - OTIMIZADO
	brutoCache = &BRUTOCache{
		data: make(map[string]interface{}),
		mu:   sync.RWMutex{},
	}

	// BRUTO Summary Response - OTIMIZADO
	brutoSummary = &BRUTOSummary{
		Default:  ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		Fallback: ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		mu:       sync.RWMutex{},
	}
)

// BRUTO Cache - OTIMIZADO
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

// BRUTO Summary with thread-safe updates - OTIMIZADO
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

	// Routes with optimized handlers
	router.HandleFunc("/summary", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		handleSummary(w, r)
	}).Methods("GET")

	// Start server with optimized settings
	server := &http.Server{
		Addr:         ":8445",
		Handler:      router,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("Summary Service BRUTO starting on :8445")
	log.Fatal(server.ListenAndServe())
}

// BRUTO: Handle summary - ULTRA-AGRESIVO
func handleSummary(w http.ResponseWriter, r *http.Request) {
	// BRUTO: Resposta hardcoded para velocidade máxima
	summary := brutoSummary.GetSummary()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"default":{"totalRequests":` + fmt.Sprintf("%d", summary.Default.TotalRequests) + `,"totalAmount":` + fmt.Sprintf("%.2f", summary.Default.TotalAmount) + `},"fallback":{"totalRequests":` + fmt.Sprintf("%d", summary.Fallback.TotalRequests) + `,"totalAmount":` + fmt.Sprintf("%.2f", summary.Fallback.TotalAmount) + `}}`))

	atomic.AddInt64(&successCount, 1)
}
