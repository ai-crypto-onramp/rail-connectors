package audit

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// Event is a normalized audit event emitted on rail state transitions.
type Event struct {
	Type       string          `json:"type"`
	PaymentID  string          `json:"payment_id"`
	Rail       string          `json:"rail"`
	Operation  string          `json:"operation,omitempty"`
	Status     string          `json:"status,omitempty"`
	Amount     decimal.Decimal `json:"amount,omitempty"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    map[string]any  `json:"payload,omitempty"`
}

// Sink is the interface every audit destination implements. The simplified
// implementation only provides an in-memory recording sink.
type Sink interface {
	Emit(Event)
}

// Recorder is an in-memory Sink that stores all emitted events for inspection
// in tests.
type Recorder struct {
	mu     sync.Mutex
	events []Event
}

// NewRecorder returns a fresh in-memory audit recorder.
func NewRecorder() *Recorder { return &Recorder{} }

// Emit records the event.
func (r *Recorder) Emit(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	r.events = append(r.events, e)
}

// Events returns a copy of all emitted events.
func (r *Recorder) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

// Count returns the number of emitted events.
func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

// Reset clears all recorded events.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}
