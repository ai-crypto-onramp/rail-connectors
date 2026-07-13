package store

import (
	"sync"
	"time"

	"github.com/ai-crypto-onramp/rail-connectors/internal/rail"
)

// Record is a single rail payment request stored in-memory.
type Record struct {
	PaymentID      string
	Rail           string
	Operation      string
	Amount         float64
	Currency       string
	Status         rail.Status
	IdempotencyKey string
	RailRef        string
	ErrorCode      string
	ErrorMessage   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SettleEntry is an in-memory settlement record.
type SettleEntry struct {
	SettleID  string
	Rail      string
	PaymentID string
	Amount    float64
	Currency  string
	SettledAt time.Time
	SourceRef string
}

// Store is a concurrency-safe in-memory rail state store. It replaces the
// PostgreSQL tables from the full spec for the simplified implementation.
type Store struct {
	mu        sync.RWMutex
	requests  map[string]*Record
	settles   []SettleEntry
	settleAmt map[string]float64 // payment_id -> total settled
}

// New constructs a new in-memory Store.
func New() *Store {
	return &Store{
		requests:  make(map[string]*Record),
		settleAmt: make(map[string]float64),
	}
}

// Upsert inserts or updates the record for PaymentID.
func (s *Store) Upsert(r Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if existing, ok := s.requests[r.PaymentID]; ok {
		r.CreatedAt = existing.CreatedAt
		r.UpdatedAt = now
		s.requests[r.PaymentID] = &r
		return
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	s.requests[r.PaymentID] = &r
}

// Get returns a copy of the record for PaymentID, or false.
func (s *Store) Get(paymentID string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.requests[paymentID]
	if !ok {
		return Record{}, false
	}
	return *r, true
}

// SetStatus updates just the status + error fields of a record.
func (s *Store) SetStatus(paymentID string, status rail.Status, code, msg string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[paymentID]
	if !ok {
		return false
	}
	r.Status = status
	r.ErrorCode = code
	r.ErrorMessage = msg
	r.UpdatedAt = time.Now().UTC()
	return true
}

// AddSettle records a settlement and marks the matching request Settled.
func (s *Store) AddSettle(e SettleEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.SettledAt = time.Now().UTC()
	if e.SettleID == "" {
		e.SettleID = "settle-" + e.PaymentID
	}
	s.settles = append(s.settles, e)
	s.settleAmt[e.PaymentID] += e.Amount
	if r, ok := s.requests[e.PaymentID]; ok {
		r.Status = rail.StatusSettled
		r.UpdatedAt = time.Now().UTC()
	}
}

// Settles returns a copy of all settlement entries.
func (s *Store) Settles() []SettleEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SettleEntry, len(s.settles))
	copy(out, s.settles)
	return out
}

// SettledAmount returns the total settled amount for a payment.
func (s *Store) SettledAmount(paymentID string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settleAmt[paymentID]
}

// All returns copies of all records (for tests/inspection).
func (s *Store) All() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, 0, len(s.requests))
	for _, r := range s.requests {
		out = append(out, *r)
	}
	return out
}
