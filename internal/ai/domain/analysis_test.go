package domain_test

import (
	"testing"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
)

func TestValidateAnalysisResult(t *testing.T) {
	valid := domain.AnalysisResult{
		Title: "Bug", Summary: "Broken button",
		Steps: []string{" Click ", "Submit"},
	}
	if err := domain.ValidateAnalysisResult(valid); err != nil {
		t.Fatal(err)
	}
	norm := domain.NormalizeAnalysisResult(valid)
	if norm.Steps[0] != "Click" {
		t.Fatalf("steps=%v", norm.Steps)
	}

	cases := []domain.AnalysisResult{
		{Title: "", Summary: "s", Steps: []string{"a"}},
		{Title: "t", Summary: "", Steps: []string{"a"}},
		{Title: "t", Summary: "s", Steps: nil},
		{Title: "t", Summary: "s", Steps: []string{"  "}},
	}
	for i, c := range cases {
		if err := domain.ValidateAnalysisResult(c); err != domain.ErrInvalidAnalysisResult {
			t.Fatalf("case %d: got %v", i, err)
		}
	}
}
