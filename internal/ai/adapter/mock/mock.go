package mock

import (
	"context"
	"fmt"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

func (a *Analyzer) Analyze(_ context.Context, in domain.AnalysisInput) (domain.AnalysisResult, error) {
	n := len(in.FrameKeys)
	return domain.AnalysisResult{
		Title:   "UI issue reproduced from recording",
		Summary: fmt.Sprintf("Mock analysis of recording %s using %d frame(s). The user encountered unexpected behavior while interacting with the app.", in.RecordingID, n),
		Steps: []string{
			"Open the application and navigate to the affected screen",
			"Perform the same actions shown in the recording",
			"Observe the unexpected result compared to the expected behavior",
		},
		Provider: "mock",
		Model:    "mock-v1",
	}, nil
}
