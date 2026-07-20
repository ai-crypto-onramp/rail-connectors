package settlement

import (
	"sync"

	"github.com/ai-crypto-onramp/rail-connectors/internal/store"
)

// Tracker keeps per-rail settlement totals in memory. It wraps the Store's
// settlement rows with convenient aggregation by rail.
type Tracker struct {
	mu     sync.Mutex
	totals map[string]float64 // rail -> total captured/settled
	store  store.Store
}

// New constructs a Tracker backed by s.
func New(s store.Store) *Tracker {
	return &Tracker{totals: map[string]float64{}, store: s}
}

// RecordCapture adds amount to the rail's captured total.
func (t *Tracker) RecordCapture(rail string, amount float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totals[rail] += amount
}

// RecordRefund subtracts amount from the rail's net total.
func (t *Tracker) RecordRefund(rail string, amount float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totals[rail] -= amount
}

// RecordSettle forwards a settlement entry to the store and aggregates it.
func (t *Tracker) RecordSettle(e store.SettleEntry) {
	t.mu.Lock()
	t.totals[e.Rail] += e.Amount
	t.mu.Unlock()
	t.store.AddSettle(e)
}

// Total returns the running net total for a rail.
func (t *Tracker) Total(rail string) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.totals[rail]
}

// Totals returns a copy of all per-rail totals.
func (t *Tracker) Totals() map[string]float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[string]float64, len(t.totals))
	for k, v := range t.totals {
		out[k] = v
	}
	return out
}
