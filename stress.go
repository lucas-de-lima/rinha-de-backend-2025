package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"
)

type PaymentRequest struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
}

func main() {
	const (
		totalRequests = 500
		concurrency   = 20
		url           = "http://localhost:9999/payments"
	)

	var (
		success    int
		timeout    int
		errorCount int
	)

	sem := make(chan struct{}, concurrency)
	wg := sync.WaitGroup{}

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			payload := PaymentRequest{
				CorrelationID: fmt.Sprintf("stress-%d-%d", time.Now().UnixNano(), i),
				Amount:        19.90,
			}
			b, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					timeout++
				} else {
					errorCount++
				}
				return
			}
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			if resp.StatusCode == 200 {
				success++
			} else {
				fmt.Printf("Erro HTTP %d: %s\n", resp.StatusCode, string(body))
				errorCount++
			}
		}(i)
	}
	wg.Wait()
	fmt.Printf("Sucesso: %d\nTimeout: %d\nErro: %d\n", success, timeout, errorCount)
}
