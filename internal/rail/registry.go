package rail

import (
	"fmt"
	"strings"
	"sync"
)

// Constructor builds a Connector for a rail family from raw config.
type Constructor func(cfg map[string]string) (Connector, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Constructor{}
)

// Register registers a constructor for a rail family. Intended to be called
// from adapter package init() functions. Panics on duplicate registration.
func Register(family string, c Constructor) {
	registryMu.Lock()
	defer registryMu.Unlock()
	f := normalize(family)
	if _, ok := registry[f]; ok {
		panic(fmt.Sprintf("rail: duplicate registration for family %q", f))
	}
	registry[f] = c
}

// New returns a Connector for the given family using the registered
// constructor. Unknown families return a normalized INVALID_REQUEST error.
func New(family string, cfg map[string]string) (Connector, error) {
	registryMu.RLock()
	c, ok := registry[normalize(family)]
	registryMu.RUnlock()
	if !ok {
		return nil, NewError(CodeInvalidRequest, fmt.Sprintf("unknown rail family %q", family))
	}
	return c(cfg)
}

// ValidFamily reports whether family is a registered rail family.
func ValidFamily(family string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[normalize(family)]
	return ok
}

// Families returns the registered family names.
func Families() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for f := range registry {
		out = append(out, f)
	}
	return out
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
