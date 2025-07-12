package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"
)

var (
	// Atomic counter for round-robin
	currentBackend int32 = 0

	// Backends - API Gateways
	backends = []string{
		"http://api-gateway-1:9999",
		"http://api-gateway-2:9999",
	}
	backendURLs []*url.URL
)

// Ultra-fast load balancer
type LoadBalancer struct{}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get next backend (round-robin)
	backend := lb.getNextBackend()

	// Create request to backend
	url := backend + r.URL.Path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequest(r.Method, url, r.Body)
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Ultra-aggressive timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// Make request to backend
	client := &http.Client{
		Timeout: 500 * time.Millisecond,
		Transport: &http.Transport{
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 200,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  true,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		// Try next backend on failure
		backend = lb.getNextBackend()
		req.URL.Host = backend
		resp, err = client.Do(req)
		if err != nil {
			http.Error(w, "Service Unavailable", 503)
			return
		}
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

func (lb *LoadBalancer) getNextBackend() string {
	next := atomic.AddInt32(&currentBackend, 1)
	return backends[next%int32(len(backends))]
}

func getNextBackend() *url.URL {
	next := atomic.AddInt32(&currentBackend, 1)
	return backendURLs[next%int32(len(backendURLs))]
}

func main() {
	// Parse backend URLs
	for _, b := range backends {
		u, err := url.Parse(b)
		if err != nil {
			log.Fatalf("Erro ao parsear backend: %v", err)
		}
		backendURLs = append(backendURLs, u)
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			backend := getNextBackend()
			req.URL.Scheme = backend.Scheme
			req.URL.Host = backend.Host
			// O Path já está correto
			// Headers já são copiados pelo ReverseProxy
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Erro no proxy: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service Unavailable"))
		},
	}

	server := &http.Server{
		Addr:    ":9999",
		Handler: proxy,
	}

	log.Printf("Load Balancer idiomático Go iniciando na porta 9999")
	log.Fatal(server.ListenAndServe())
}
