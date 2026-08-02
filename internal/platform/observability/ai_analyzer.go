package observability

import (
	"context"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
	"github.com/Brohammad/BugSathi/internal/ai/port"
)

// Analyzer wraps an Analyzer with latency metrics.
type Analyzer struct {
	Inner   port.Analyzer
	Metrics *Metrics
	Name    string
}

func (a Analyzer) Analyze(ctx context.Context, in domain.AnalysisInput) (domain.AnalysisResult, error) {
	start := time.Now()
	out, err := a.Inner.Analyze(ctx, in)
	provider := a.Name
	if out.Provider != "" {
		provider = out.Provider
	}
	if a.Metrics != nil {
		a.Metrics.ObserveAI(provider, err, time.Since(start))
	}
	return out, err
}
