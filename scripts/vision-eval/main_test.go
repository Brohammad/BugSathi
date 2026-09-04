package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
)

func TestScoreRequiresGroundedFactsAndRejectsForbiddenClaims(t *testing.T) {
	tc := evalCase{
		RequiredFacts:   []string{"73%", "No progress for 5 minutes"},
		ForbiddenClaims: []string{"upload completed"},
	}
	report := domain.AnalysisResult{
		Title:   "Upload is stuck at 73%",
		Summary: "No progress for 5 minutes in the browser.",
		Steps:   []string{"Open the upload page"},
	}
	matched, missing, forbidden, got := score(tc, report)
	if len(matched) != 2 || len(missing) != 0 || len(forbidden) != 0 || got != 10 {
		t.Fatalf("matched=%v missing=%v forbidden=%v score=%d", matched, missing, forbidden, got)
	}

	report.Summary += " The upload completed."
	_, _, forbidden, got = score(tc, report)
	if len(forbidden) != 1 || got != 7 {
		t.Fatalf("forbidden=%v score=%d", forbidden, got)
	}
}

func TestLoadAIEnvOnlyLoadsAIPrefixWithoutOverwritingProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	err := os.WriteFile(path, []byte(
		"AI_API_KEY=file-key\n"+
			"AI_MODEL=\"vision-model\"\n"+
			"POSTGRES_PASSWORD=must-not-load\n",
	), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AI_API_KEY", "process-key")
	t.Setenv("AI_MODEL", "")
	t.Setenv("POSTGRES_PASSWORD", "")

	if err := loadAIEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("AI_API_KEY"); got != "process-key" {
		t.Fatalf("AI_API_KEY=%q", got)
	}
	if got := os.Getenv("AI_MODEL"); got != "vision-model" {
		t.Fatalf("AI_MODEL=%q", got)
	}
	if got := os.Getenv("POSTGRES_PASSWORD"); got != "" {
		t.Fatalf("POSTGRES_PASSWORD unexpectedly loaded")
	}
}
