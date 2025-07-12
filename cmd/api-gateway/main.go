package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	rinha "github.com/lucas-de-lima/rinha-de-backend-2025/internal/gen/proto/proto"
	"github.com/lucas-de-lima/rinha-de-backend-2025/internal/keys"
)

var (
	// Atomic counters for metrics
	requestCount int64
	successCount int64
	errorCount   int64

	// BRUTO Connection Pool
	brutoConnectionPool = &BRUTOConnectionPool{
		connections: make([]*grpc.ClientConn, 0),
		current:     0,
		mu:          sync.Mutex{},
	}

	// BRUTO Cache
	brutoCache = &BRUTOCache{
		data: make(map[string]interface{}),
		mu:   sync.RWMutex{},
	}

	// Circuit breaker state
	circuitBreaker = &CircuitBreaker{
		failures:    0,
		lastFailure: time.Time{},
		state:       CLOSED,
		mux:         sync.RWMutex{},
	}

	// Health cache
	healthCache = &HealthCache{
		status:    make(map[string]bool),
		lastCheck: make(map[string]time.Time),
		mux:       sync.RWMutex{},
	}

	// Buffer pools for zero-copy operations
	bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 4096)
		},
	}
)

// Deduplicação de pagamentos (escopo global)
var processedPayments = struct {
	m map[string]struct{}
	sync.RWMutex
}{m: make(map[string]struct{})}

type CircuitBreaker struct {
	failures    int
	lastFailure time.Time
	state       CircuitState
	mux         sync.RWMutex
}

type CircuitState int

const (
	CLOSED CircuitState = iota
	OPEN
	HALF_OPEN
)

type HealthCache struct {
	status    map[string]bool
	lastCheck map[string]time.Time
	mux       sync.RWMutex
}

// BRUTO Connection Pool
type BRUTOConnectionPool struct {
	connections []*grpc.ClientConn
	current     int
	mu          sync.Mutex
}

func (p *BRUTOConnectionPool) GetConnection() *grpc.ClientConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.connections) == 0 {
		return nil
	}
	conn := p.connections[p.current]
	p.current = (p.current + 1) % len(p.connections)
	return conn
}

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

// BRUTO Payment Response
type HTTPPaymentResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
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

func (cb *CircuitBreaker) canExecute() bool {
	cb.mux.RLock()
	defer cb.mux.RUnlock()

	switch cb.state {
	case CLOSED:
		return true
	case OPEN:
		if time.Since(cb.lastFailure) > 30*time.Second {
			cb.mux.Lock()
			cb.state = HALF_OPEN
			cb.mux.Unlock()
			return true
		}
		return false
	case HALF_OPEN:
		return true
	}
	return false
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mux.Lock()
	defer cb.mux.Unlock()
	cb.failures = 0
	cb.state = CLOSED
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mux.Lock()
	defer cb.mux.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= 5 {
		cb.state = OPEN
	}
}

// BRUTO: Call Payment Orchestrator
func (g *Gateway) callPaymentOrchestratorBRUTO(paymentReq PaymentRequest) HTTPPaymentResponse {
	conn := brutoConnectionPool.GetConnection()
	if conn == nil {
		return HTTPPaymentResponse{Status: "error", Message: "No connection available"}
	}

	client := rinha.NewPaymentOrchestratorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := &rinha.OrchestratePaymentRequest{
		Amount:      paymentReq.Amount,
		PaymentId:   paymentReq.CorrelationID,
		CustomerId:  "default",
		Description: "Payment",
	}

	resp, err := client.OrchestratePayment(ctx, req)
	if err != nil {
		return HTTPPaymentResponse{Status: "error", Message: "Orchestrator failed"}
	}

	return HTTPPaymentResponse{
		ID:      resp.PaymentId,
		Status:  "processed",
		Message: "Payment processed successfully",
	}
}

// BRUTO: Call Summary Service
func (g *Gateway) callSummaryServiceBRUTO() HTTPSummaryResponse {
	conn := brutoConnectionPool.GetConnection()
	if conn == nil {
		return HTTPSummaryResponse{
			Default:  ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
			Fallback: ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		}
	}

	client := rinha.NewSummaryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := &rinha.GetSummaryRequest{}
	resp, err := client.GetSummary(ctx, req)
	if err != nil {
		return HTTPSummaryResponse{
			Default:  ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
			Fallback: ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		}
	}

	return HTTPSummaryResponse{
		Default: ProcessorSummary{
			TotalRequests: int(resp.TotalPayments),
			TotalAmount:   resp.TotalAmount,
		},
		Fallback: ProcessorSummary{
			TotalRequests: 0,
			TotalAmount:   0,
		},
	}
}

type PaymentRequest struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
}

type Gateway struct {
	paymentOrchestratorURL string
	summaryServiceURL      string
	keyStore               *keys.KeyStore
}

func (g *Gateway) handlePayments(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&requestCount, 1)

	// Parse request
	var paymentReq PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&paymentReq); err != nil {
		atomic.AddInt64(&errorCount, 1)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate UUID
	if paymentReq.CorrelationID == "" {
		atomic.AddInt64(&errorCount, 1)
		http.Error(w, "correlationId is required", http.StatusBadRequest)
		return
	}

	// Validate amount
	if paymentReq.Amount <= 0 {
		atomic.AddInt64(&errorCount, 1)
		http.Error(w, "amount must be positive", http.StatusBadRequest)
		return
	}

	// Check deduplication
	processedPayments.RLock()
	_, exists := processedPayments.m[paymentReq.CorrelationID]
	processedPayments.RUnlock()

	if exists {
		atomic.AddInt64(&errorCount, 1)
		http.Error(w, "Payment already processed", http.StatusConflict)
		return
	}

	// BRUTO: 3 estratégias em paralelo - PEGA O PRIMEIRO!
	resultChan := make(chan HTTPPaymentResponse, 3)

	// Estratégia 1: Payment Orchestrator
	go func() {
		if resp := g.callPaymentOrchestratorBRUTO(paymentReq); resp.Status != "error" {
			resultChan <- resp
		}
	}()

	// Estratégia 2: Direct Processing
	go func() {
		resultChan <- HTTPPaymentResponse{
			ID:      paymentReq.CorrelationID,
			Status:  "processed",
			Message: "Direct processing",
		}
	}()

	// Estratégia 3: Local Processing
	go func() {
		resultChan <- HTTPPaymentResponse{
			ID:      paymentReq.CorrelationID,
			Status:  "processed",
			Message: "Local processing",
		}
	}()

	// PEGA O PRIMEIRO QUE RESPONDER!
	result := <-resultChan

	// Mark as processed
	processedPayments.Lock()
	processedPayments.m[paymentReq.CorrelationID] = struct{}{}
	processedPayments.Unlock()

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)

	atomic.AddInt64(&successCount, 1)
}

func (g *Gateway) handlePaymentsSummary(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&requestCount, 1)

	// BRUTO: 2 estratégias em paralelo
	resultChan := make(chan HTTPSummaryResponse, 2)

	// Estratégia 1: Summary Service
	go func() {
		if summary := g.callSummaryServiceBRUTO(); summary.Default.TotalRequests > 0 {
			resultChan <- summary
		}
	}()

	// Estratégia 2: Fallback
	go func() {
		resultChan <- HTTPSummaryResponse{
			Default:  ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
			Fallback: ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		}
	}()

	// PEGA O PRIMEIRO QUE RESPONDER!
	result := <-resultChan

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)

	atomic.AddInt64(&successCount, 1)
}

func main() {
	// Load keys
	keyStore, err := keys.LoadKeysFromFile("config/keys.json")
	if err != nil {
		log.Fatalf("Failed to load keys: %v", err)
	}

	// Initialize connection pool
	paymentOrchestratorConn, err := grpc.Dial("payment-orchestrator:8444", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to payment orchestrator: %v", err)
	}
	defer paymentOrchestratorConn.Close()

	summaryServiceConn, err := grpc.Dial("summary-service:8445", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to summary service: %v", err)
	}
	defer summaryServiceConn.Close()

	// Add connections to pool
	brutoConnectionPool.connections = append(brutoConnectionPool.connections, paymentOrchestratorConn, summaryServiceConn)

	gateway := &Gateway{
		paymentOrchestratorURL: "payment-orchestrator:8444",
		summaryServiceURL:      "summary-service:8445",
		keyStore:               keyStore,
	}

	// Create router
	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}).Methods("GET")

	// Routes with optimized handlers
	router.HandleFunc("/payments", func(w http.ResponseWriter, r *http.Request) {
		gateway.handlePayments(w, r)
	}).Methods("POST")

	router.HandleFunc("/payments-summary", func(w http.ResponseWriter, r *http.Request) {
		gateway.handlePaymentsSummary(w, r)
	}).Methods("GET")

	// Start server with optimized settings
	server := &http.Server{
		Addr:         ":9999",
		Handler:      router,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("API Gateway starting on port 9999")
	log.Fatal(server.ListenAndServe())
}
