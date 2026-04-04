package forex

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"c2c_monitor/internal/domain"
)

type namedForex interface {
	domain.IForex
	SourceName() string
}

// FallbackAdapter tries sources in order and records the last successful source.
type FallbackAdapter struct {
	sources    []namedForex
	lastSource string
	mu         sync.RWMutex
}

func NewFallbackAdapter(sources ...namedForex) *FallbackAdapter {
	filtered := make([]namedForex, 0, len(sources))
	for _, source := range sources {
		if source != nil {
			filtered = append(filtered, source)
		}
	}

	return &FallbackAdapter{sources: filtered}
}

func (a *FallbackAdapter) GetRate(ctx context.Context, from, to string) (float64, error) {
	var errs []string
	for _, source := range a.sources {
		rate, err := source.GetRate(ctx, from, to)
		if err == nil {
			a.setLastSource(source.SourceName())
			return rate, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", source.SourceName(), err))
	}

	a.setLastSource("")
	if len(errs) == 0 {
		return 0, fmt.Errorf("no forex sources configured")
	}
	return 0, fmt.Errorf("all forex sources failed: %s", strings.Join(errs, "; "))
}

func (a *FallbackAdapter) LastSourceName() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastSource
}

func (a *FallbackAdapter) setLastSource(source string) {
	a.mu.Lock()
	a.lastSource = source
	a.mu.Unlock()
}
