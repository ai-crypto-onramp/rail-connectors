package metrics

import (
	"strings"
	"testing"
)

func TestCounterIncAdd(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	c := r.RegisterCounter("rail_test_total", "test counter")
	c.Inc()
	c.Add(4)
	if c.Value() != 5 {
		t.Fatalf("value = %v", c.Value())
	}
	if r.Counter("rail_test_total") != c {
		t.Fatal("lookup returned different counter")
	}
}

func TestHistogramObserve(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	h := r.RegisterHistogram("rail_test_latency", "latency", []float64{0.1, 1, 10})
	h.Observe(0.05)
	h.Observe(0.5)
	h.Observe(5)
	h.Observe(50)
	if r.Histogram("rail_test_latency") != h {
		t.Fatal("lookup returned different histogram")
	}
}

func TestRenderContainsAll(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.RegisterCounter("rail_a_total", "a").Inc()
	r.RegisterHistogram("rail_b_latency", "b", []float64{1}).Observe(1)
	out := r.Render()
	if !strings.Contains(out, "rail_a_total") {
		t.Fatal("missing counter")
	}
	if !strings.Contains(out, "rail_b_latency_bucket") {
		t.Fatal("missing histogram buckets")
	}
	if !strings.Contains(out, "le=\"+Inf\"") {
		t.Fatal("missing +Inf bucket")
	}
}

func TestRegisterIdempotent(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	c1 := r.RegisterCounter("dup", "x")
	c2 := r.RegisterCounter("dup", "x")
	if c1 != c2 {
		t.Fatal("duplicate register returned new counter")
	}
	h1 := r.RegisterHistogram("hdup", "x", []float64{1})
	h2 := r.RegisterHistogram("hdup", "x", []float64{1})
	if h1 != h2 {
		t.Fatal("duplicate register returned new histogram")
	}
}
