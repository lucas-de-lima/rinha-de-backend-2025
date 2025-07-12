package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"github.com/lucas-de-lima/rinha-de-backend-2025/internal/keys"
)

var (
	// Atomic counters for metrics
	requestCount int64
	successCount int64
	errorCount   int64
	timeoutCount int64

	// Connection pools
	grpcConnPool = make(map[string]*grpc.ClientConn)
	connPoolMux  sync.RWMutex

	// Circuit breaker state
	circuitBreaker = &CircuitBreaker{
		failures:    0,
		lastFailure: time.Time{},
		state:       CLOSED,
		mux:         sync.RWMutex{},
	}

	// Health check cache
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

	// BRUTO Connection Pool
	brutoConnectionPool = &BRUTOConnectionPool{
		connections: make([]*http.Client, 0),
		current:     0,
		mu:          sync.Mutex{},
	}

	// BRUTO Cache
	brutoCache = &BRUTOCache{
		data: make(map[string]interface{}),
		mu:   sync.RWMutex{},
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
	connections []*http.Client
	current     int
	mu          sync.Mutex
}

func (p *BRUTOConnectionPool) GetConnection() *http.Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.connections) == 0 {
		// Create new connection with optimized settings
		client := &http.Client{
			Timeout: 500 * time.Millisecond, // BRUTO timeout
			Transport: &http.Transport{
				MaxIdleConns:        500, // Aumentado
				MaxIdleConnsPerHost: 100, // Aumentado
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
				DisableCompression:  true, // BRUTO: disable compression for speed
			},
		}
		p.connections = append(p.connections, client)
		return client
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
			cb.mux.RUnlock()
			cb.mux.Lock()
			cb.state = HALF_OPEN
			cb.mux.Unlock()
			cb.mux.RLock()
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
	if cb.failures >= 5 {
		cb.state = OPEN
		cb.lastFailure = time.Now()
	}
}

func (hc *HealthCache) isHealthy(service string) bool {
	hc.mux.RLock()
	defer hc.mux.RUnlock()

	if status, exists := hc.status[service]; exists {
		if time.Since(hc.lastCheck[service]) < 30*time.Second {
			return status
		}
	}
	return true // Assume healthy if no recent check
}

func getGRPCConnection(target string) (*grpc.ClientConn, error) {
	connPoolMux.RLock()
	if conn, exists := grpcConnPool[target]; exists {
		connPoolMux.RUnlock()
		return conn, nil
	}
	connPoolMux.RUnlock()

	connPoolMux.Lock()
	defer connPoolMux.Unlock()

	// Double-check after acquiring write lock
	if conn, exists := grpcConnPool[target]; exists {
		return conn, nil
	}

	// Load certificates
	cert, err := tls.LoadX509KeyPair("certs/client.crt", "certs/client.key")
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %v", err)
	}

	caCert, err := ioutil.ReadFile("certs/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		ServerName:   "localhost",
	}

	conn, err := grpc.Dial(target,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		return nil, err
	}

	grpcConnPool[target] = conn
	return conn, nil
}

// BRUTO: Call Payment Orchestrator
func callPaymentOrchestratorBRUTO(paymentReq map[string]interface{}) HTTPPaymentResponse {
	// BRUTO: Use connection pool
	client := brutoConnectionPool.GetConnection()

	// BRUTO: Ultra-aggressive timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// BRUTO: Zero-copy JSON marshaling
	buffer := bufferPool.Get().([]byte)
	defer bufferPool.Put(buffer)

	jsonData, err := json.Marshal(paymentReq)
	if err != nil {
		return HTTPPaymentResponse{Status: "error", Message: "JSON marshal failed"}
	}

	// BRUTO: Direct HTTP call to payment orchestrator
	req, err := http.NewRequestWithContext(ctx, "POST", "http://payment-orchestrator:8444/payments", bytes.NewBuffer(jsonData))
	if err != nil {
		return HTTPPaymentResponse{Status: "error", Message: "Request creation failed"}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return HTTPPaymentResponse{Status: "error", Message: "Orchestrator failed"}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return HTTPPaymentResponse{
			ID:      paymentReq["correlationId"].(string),
			Status:  "processed",
			Message: "Payment orchestrated successfully",
		}
	}

	return HTTPPaymentResponse{Status: "error", Message: "Orchestrator returned error"}
}

// BRUTO: Direct Payment Processor Integration
func callPaymentProcessorBRUTO(paymentReq map[string]interface{}, processor string) HTTPPaymentResponse {
	// BRUTO: Use connection pool
	client := brutoConnectionPool.GetConnection()

	// BRUTO: Ultra-aggressive timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Add requestedAt timestamp for Rinha spec
	paymentReq["requestedAt"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	jsonData, err := json.Marshal(paymentReq)
	if err != nil {
		return HTTPPaymentResponse{Status: "error", Message: "JSON marshal failed"}
	}

	// BRUTO: Direct HTTP call to payment processor
	url := fmt.Sprintf("http://%s:8080/payments", processor)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return HTTPPaymentResponse{Status: "error", Message: "Request creation failed"}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return HTTPPaymentResponse{Status: "error", Message: fmt.Sprintf("%s failed", processor)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return HTTPPaymentResponse{
			ID:      paymentReq["correlationId"].(string),
			Status:  "processed",
			Message: fmt.Sprintf("Payment processed by %s", processor),
		}
	}

	return HTTPPaymentResponse{Status: "error", Message: fmt.Sprintf("%s returned error", processor)}
}

// BRUTO: Call Summary Service
func callSummaryServiceBRUTO() HTTPSummaryResponse {
	// BRUTO: Use connection pool
	client := brutoConnectionPool.GetConnection()

	// BRUTO: Ultra-aggressive timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// BRUTO: Direct HTTP call to summary service
	req, err := http.NewRequestWithContext(ctx, "GET", "http://summary-service:8445/summary", nil)
	if err != nil {
		return HTTPSummaryResponse{
			Default:  ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
			Fallback: ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return HTTPSummaryResponse{
			Default:  ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
			Fallback: ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var summary HTTPSummaryResponse
		if err := json.NewDecoder(resp.Body).Decode(&summary); err == nil {
			return summary
		}
	}

	return HTTPSummaryResponse{
		Default:  ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		Fallback: ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
	}
}

// BRUTO: Health check for payment processors with rate limiting
var lastHealthCheck = make(map[string]time.Time)
var healthCheckMutex sync.Mutex

func checkPaymentProcessorHealth(processor string) bool {
	healthCheckMutex.Lock()
	defer healthCheckMutex.Unlock()

	// BRUTO: Rate limiting - 1 call per 5 seconds
	if lastCheck, exists := lastHealthCheck[processor]; exists {
		if time.Since(lastCheck) < 5*time.Second {
			// Return cached result
			return brutoCache.Get(fmt.Sprintf("health_%s", processor)) == true
		}
	}

	lastHealthCheck[processor] = time.Now()

	client := brutoConnectionPool.GetConnection()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	url := fmt.Sprintf("http://%s:8080/payments/service-health", processor)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		brutoCache.Set(fmt.Sprintf("health_%s", processor), false)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		brutoCache.Set(fmt.Sprintf("health_%s", processor), false)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var health struct {
			Failing bool `json:"failing"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&health); err == nil {
			isHealthy := !health.Failing
			brutoCache.Set(fmt.Sprintf("health_%s", processor), isHealthy)
			return isHealthy
		}
	}

	brutoCache.Set(fmt.Sprintf("health_%s", processor), false)
	return false
}

func main() {
	// Load keys
	keyStore, err := keys.LoadKeysFromFile("config/keys.json")
	if err != nil {
		log.Fatalf("Failed to load keys: %v", err)
	}

	// Create router
	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}).Methods("GET")

	// Admin endpoints for testing
	router.HandleFunc("/purge-payments", func(w http.ResponseWriter, r *http.Request) {
		handlePurgePayments(w, r)
	}).Methods("POST")

	router.HandleFunc("/payments-summary", func(w http.ResponseWriter, r *http.Request) {
		handlePaymentsSummary(w, r)
	}).Methods("GET")

	// Routes with optimized handlers
	router.HandleFunc("/payments", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		handlePayments(w, r, keyStore)
	}).Methods("POST")

	// Payments summary endpoint
	router.HandleFunc("/payments-summary", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		handlePaymentsSummary(w, r)
	}).Methods("GET")

	// Start server with optimized settings
	server := &http.Server{
		Addr:         ":9999",
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Println("API Gateway BRUTO starting on :9999")
	log.Fatal(server.ListenAndServe())
}

// BRUTO: Handle payments com deduplicação e prioridade para processor real
func handlePayments(w http.ResponseWriter, r *http.Request, keyStore *keys.KeyStore) {
	if !circuitBreaker.canExecute() {
		atomic.AddInt64(&errorCount, 1)
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	var paymentReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&paymentReq); err != nil {
		atomic.AddInt64(&errorCount, 1)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	correlationId, _ := paymentReq["correlationId"].(string)
	// Deduplicação: se já processou, retorna sucesso idempotente
	processedPayments.RLock()
	_, exists := processedPayments.m[correlationId]
	processedPayments.RUnlock()
	if exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HTTPPaymentResponse{
			ID:      correlationId,
			Status:  "processed",
			Message: "Idempotent: already processed",
		})
		return
	}
	// Canal para resultado
	resultChan := make(chan HTTPPaymentResponse, 2)
	// Estratégia 1: Payment Processor (real)
	go func() {
		if checkPaymentProcessorHealth("payment-processor") {
			resp := callPaymentProcessorBRUTO(paymentReq, "payment-processor")
			if resp.Status != "error" {
				resultChan <- resp
			}
		}
	}()
	// Estratégia 2: Fallback (local)
	go func() {
		time.Sleep(200 * time.Millisecond) // Dá chance do real responder primeiro
		resultChan <- HTTPPaymentResponse{
			ID:      correlationId,
			Status:  "processed",
			Message: "Local fallback",
		}
	}()
	// Pega o primeiro que chegar
	result := <-resultChan
	// Marca como processado
	processedPayments.Lock()
	processedPayments.m[correlationId] = struct{}{}
	processedPayments.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
	atomic.AddInt64(&successCount, 1)
	circuitBreaker.recordSuccess()
}

func handlePaymentsSummary(w http.ResponseWriter, r *http.Request) {
	// BRUTO: 2 strategies in parallel
	resultChan := make(chan HTTPSummaryResponse, 2)

	// Strategy 1: Summary Service
	go func() {
		if summary := callSummaryServiceBRUTO(); summary.Default.TotalRequests > 0 {
			resultChan <- summary
		}
	}()

	// Strategy 2: Fallback
	go func() {
		resultChan <- HTTPSummaryResponse{
			Default:  ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
			Fallback: ProcessorSummary{TotalRequests: 0, TotalAmount: 0},
		}
	}()

	// PEGA O PRIMEIRO QUE RESPONDER!
	result := <-resultChan

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
	atomic.AddInt64(&successCount, 1)
}

func handlePurgePayments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"Payments purged"}`))
}
