package database

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"sort"
	"time"

	goBolt "go.etcd.io/bbolt"
)

// Payment representa um pagamento no banco de dados
type Payment struct {
	ID            string    `json:"id"`
	CustomerID    string    `json:"customer_id"`
	Amount        float64   `json:"amount"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	ProcessorUsed string    `json:"processor_used"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Database representa a conexão com o banco de dados
// Agora usa BoltDB
type Database struct {
	db *goBolt.DB
}

const paymentsBucket = "payments"

// NewDatabase cria uma nova conexão com o banco BoltDB
func NewDatabase(dbPath string) (*Database, error) {
	db, err := goBolt.Open(dbPath, 0600, &goBolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir banco BoltDB: %w", err)
	}
	// Cria bucket se não existir
	err = db.Update(func(tx *goBolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(paymentsBucket))
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("erro ao criar bucket: %w", err)
	}
	return &Database{db: db}, nil
}

// Close fecha a conexão com o banco de dados
func (d *Database) Close() error {
	return d.db.Close()
}

// CreatePayment insere um novo pagamento no banco BoltDB
func (d *Database) CreatePayment(payment *Payment) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(payment); err != nil {
		return fmt.Errorf("erro ao serializar pagamento: %w", err)
	}
	key := []byte(payment.ID)
	err := d.db.Update(func(tx *goBolt.Tx) error {
		bucket := tx.Bucket([]byte(paymentsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s não existe", paymentsBucket)
		}
		return bucket.Put(key, buf.Bytes())
	})
	if err != nil {
		return fmt.Errorf("erro ao inserir pagamento: %w", err)
	}
	log.Printf("[database] Pagamento criado: ID=%s, Customer=%s, Amount=%.2f", payment.ID, payment.CustomerID, payment.Amount)
	return nil
}

// UpdatePayment atualiza um pagamento existente
func (d *Database) UpdatePayment(payment *Payment) error {
	key := []byte(payment.ID)
	return d.db.Update(func(tx *goBolt.Tx) error {
		bucket := tx.Bucket([]byte(paymentsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s não existe", paymentsBucket)
		}
		// Busca o pagamento atual
		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("pagamento não encontrado: %s", payment.ID)
		}
		var existing Payment
		if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&existing); err != nil {
			return fmt.Errorf("erro ao decodificar pagamento: %w", err)
		}
		// Atualiza campos
		existing.Status = payment.Status
		existing.ProcessorUsed = payment.ProcessorUsed
		existing.UpdatedAt = payment.UpdatedAt
		// Serializa novamente
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(&existing); err != nil {
			return fmt.Errorf("erro ao serializar pagamento: %w", err)
		}
		return bucket.Put(key, buf.Bytes())
	})
}

// GetPaymentByID busca um pagamento pelo ID
func (d *Database) GetPaymentByID(id string) (*Payment, error) {
	key := []byte(id)
	var payment *Payment
	err := d.db.View(func(tx *goBolt.Tx) error {
		bucket := tx.Bucket([]byte(paymentsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s não existe", paymentsBucket)
		}
		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("pagamento não encontrado: %s", id)
		}
		var p Payment
		if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&p); err != nil {
			return fmt.Errorf("erro ao decodificar pagamento: %w", err)
		}
		payment = &p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return payment, nil
}

// GetPaymentsByCustomer busca pagamentos por cliente
func (d *Database) GetPaymentsByCustomer(customerID string) ([]*Payment, error) {
	var payments []*Payment
	err := d.db.View(func(tx *goBolt.Tx) error {
		bucket := tx.Bucket([]byte(paymentsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s não existe", paymentsBucket)
		}
		return bucket.ForEach(func(k, v []byte) error {
			var p Payment
			if err := gob.NewDecoder(bytes.NewReader(v)).Decode(&p); err != nil {
				return err
			}
			if p.CustomerID == customerID {
				payments = append(payments, &p)
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar pagamentos: %w", err)
	}
	// Ordena por CreatedAt DESC
	sort.Slice(payments, func(i, j int) bool {
		return payments[i].CreatedAt.After(payments[j].CreatedAt)
	})
	return payments, nil
}

// GetPaymentSummary calcula o resumo de pagamentos por cliente
func (d *Database) GetPaymentSummary(customerID string) (float64, int, error) {
	var totalAmount float64
	var count int
	err := d.db.View(func(tx *goBolt.Tx) error {
		bucket := tx.Bucket([]byte(paymentsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s não existe", paymentsBucket)
		}
		return bucket.ForEach(func(k, v []byte) error {
			var p Payment
			if err := gob.NewDecoder(bytes.NewReader(v)).Decode(&p); err != nil {
				return err
			}
			if p.CustomerID == customerID && p.Status == "completed" {
				totalAmount += p.Amount
				count++
			}
			return nil
		})
	})
	if err != nil {
		return 0, 0, fmt.Errorf("erro ao calcular resumo: %w", err)
	}
	return totalAmount, count, nil
}

// GetPaymentStats retorna estatísticas gerais dos pagamentos
func (d *Database) GetPaymentStats() (map[string]interface{}, error) {
	var totalPayments, completedPayments, processingPayments, errorPayments, uniqueCustomers int
	var totalAmount float64
	customerSet := make(map[string]struct{})
	err := d.db.View(func(tx *goBolt.Tx) error {
		bucket := tx.Bucket([]byte(paymentsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s não existe", paymentsBucket)
		}
		return bucket.ForEach(func(k, v []byte) error {
			var p Payment
			if err := gob.NewDecoder(bytes.NewReader(v)).Decode(&p); err != nil {
				return err
			}
			totalPayments++
			customerSet[p.CustomerID] = struct{}{}
			switch p.Status {
			case "completed":
				completedPayments++
				totalAmount += p.Amount
			case "processing":
				processingPayments++
			case "error":
				errorPayments++
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar estatísticas: %w", err)
	}
	uniqueCustomers = len(customerSet)
	stats := map[string]interface{}{
		"total_payments":      totalPayments,
		"completed_payments":  completedPayments,
		"processing_payments": processingPayments,
		"error_payments":      errorPayments,
		"total_amount":        totalAmount,
		"unique_customers":    uniqueCustomers,
	}
	return stats, nil
}

// CleanupOldPayments remove pagamentos antigos (opcional, para manutenção)
func (d *Database) CleanupOldPayments(daysOld int) error {
	limite := time.Now().AddDate(0, 0, -daysOld)
	var removidos int
	err := d.db.Update(func(tx *goBolt.Tx) error {
		bucket := tx.Bucket([]byte(paymentsBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s não existe", paymentsBucket)
		}
		var keysToDelete [][]byte
		err := bucket.ForEach(func(k, v []byte) error {
			var p Payment
			if err := gob.NewDecoder(bytes.NewReader(v)).Decode(&p); err != nil {
				return err
			}
			if p.CreatedAt.Before(limite) {
				keysToDelete = append(keysToDelete, append([]byte{}, k...))
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, k := range keysToDelete {
			if err := bucket.Delete(k); err == nil {
				removidos++
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("erro ao limpar pagamentos antigos: %w", err)
	}
	log.Printf("[database] %d pagamentos antigos removidos", removidos)
	return nil
}
 