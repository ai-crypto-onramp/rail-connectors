// Package metrics is a minimal in-process Prometheus-compatible metrics
// registry. It exposes Counters and Histograms that can be rendered in the
// Prometheus text exposition format for the /metrics endpoint.
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry is a concurrency-safe collection of metrics.
type Registry struct {
	mu        sync.RWMutex
	counters  map[string]*Counter
	histogram map[string]*Histogram
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:  map[string]*Counter{},
		histogram: map[string]*Histogram{},
	}
}

// Default is the process-wide registry used by adapters that register their
// metrics at package init time.
var Default = NewRegistry()

// RegisterCounter registers a counter; duplicate names return the existing
// counter (so multiple init() calls are safe).
func (r *Registry) RegisterCounter(name, help string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &Counter{name: name, help: help}
	r.counters[name] = c
	return c
}

// RegisterHistogram registers a histogram with the given cumulative upper
// bounds (le buckets). Duplicate names return the existing histogram.
func (r *Registry) RegisterHistogram(name, help string, buckets []float64) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histogram[name]; ok {
		return h
	}
	h := &Histogram{name: name, help: help, buckets: append([]float64(nil), buckets...)}
	h.counts = make([]uint64, len(h.buckets)+1)
	r.histogram[name] = h
	return h
}

// Counter returns a registered counter by name, or nil.
func (r *Registry) Counter(name string) *Counter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.counters[name]
}

// Histogram returns a registered histogram by name, or nil.
func (r *Registry) Histogram(name string) *Histogram {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.histogram[name]
}

// Render emits all registered metrics in Prometheus text exposition format.
func (r *Registry) Render() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var b strings.Builder
	names := make([]string, 0, len(r.counters)+len(r.histogram))
	for n := range r.counters {
		names = append(names, n)
	}
	for n := range r.histogram {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if c, ok := r.counters[n]; ok {
			c.render(&b)
		} else if h, ok := r.histogram[n]; ok {
			h.render(&b)
		}
	}
	return b.String()
}

// Counter is a monotonically increasing float64 counter.
type Counter struct {
	name  string
	help  string
	mu    sync.Mutex
	value float64
}

// Inc adds 1 to the counter.
func (c *Counter) Inc() { c.Add(1) }

// Add adds v (v >= 0) to the counter.
func (c *Counter) Add(v float64) {
	c.mu.Lock()
	c.value += v
	c.mu.Unlock()
}

// Value returns the current counter value.
func (c *Counter) Value() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func (c *Counter) render(b *strings.Builder) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s counter\n", c.name, c.help, c.name)
	c.mu.Lock()
	v := c.value
	c.mu.Unlock()
	fmt.Fprintf(b, "%s %g\n", c.name, v)
}

// Histogram tracks observations bucketed by cumulative upper bounds.
type Histogram struct {
	name    string
	help    string
	buckets []float64
	mu      sync.Mutex
	counts  []uint64 // len = len(buckets)+1, last bucket is +Inf
	sum     float64
	count   uint64
}

// Observe records a single observation.
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	h.sum += v
	h.count++
	for i, le := range h.buckets {
		if v <= le {
			h.counts[i]++
		}
	}
	h.counts[len(h.counts)-1]++ // +Inf bucket
	h.mu.Unlock()
}

func (h *Histogram) render(b *strings.Builder) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s histogram\n", h.name, h.help, h.name)
	h.mu.Lock()
	sum, count := h.sum, h.count
	counts := append([]uint64(nil), h.counts...)
	h.mu.Unlock()
	for i, le := range h.buckets {
		fmt.Fprintf(b, "%s_bucket{le=\"%g\"} %d\n", h.name, le, counts[i])
	}
	fmt.Fprintf(b, "%s_bucket{le=\"+Inf\"} %d\n", h.name, counts[len(counts)-1])
	fmt.Fprintf(b, "%s_sum %g\n%s_count %d\n", h.name, sum, h.name, count)
}