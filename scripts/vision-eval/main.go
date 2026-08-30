package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/adapter/openai"
	"github.com/Brohammad/BugSathi/internal/ai/domain"
)

type evalCase struct {
	ID              string   `json:"id"`
	PrimaryDefect   string   `json:"primary_defect"`
	RequiredFacts   []string `json:"required_facts"`
	ForbiddenClaims []string `json:"forbidden_claims"`
	Frames          []string `json:"frames"`
}

type evalResult struct {
	ID                 string                `json:"id"`
	Model              string                `json:"model"`
	PromptVersion      string                `json:"prompt_version"`
	FrameCount         int                   `json:"frame_count"`
	LatencyMS          int64                 `json:"latency_ms"`
	Report             domain.AnalysisResult `json:"report"`
	MatchedFacts       []string              `json:"matched_facts"`
	MissingFacts       []string              `json:"missing_facts"`
	ForbiddenMatches   []string              `json:"forbidden_matches"`
	AutomatedScore     int                   `json:"automated_score"`
	AutomatedPass      bool                  `json:"automated_pass"`
	ManualReviewNeeded bool                  `json:"manual_review_needed"`
	Error              string                `json:"error,omitempty"`
}

type evalRun struct {
	GeneratedAt     time.Time    `json:"generated_at"`
	Model           string       `json:"model"`
	PromptVersion   string       `json:"prompt_version"`
	Cases           []evalResult `json:"cases"`
	AutomatedPassed int          `json:"automated_passed"`
	Total           int          `json:"total"`
	ReleaseGateMet  bool         `json:"release_gate_met"`
	ScoringNote     string       `json:"scoring_note"`
}

func main() {
	var (
		envPath      = flag.String("env", ".env", "dotenv file containing AI settings")
		manifestPath = flag.String("manifest", "docs/evaluation/fixtures/cases.json", "evaluation case manifest")
		imagesPath   = flag.String("images", "docs/evaluation/fixtures/images", "fixture image directory")
		outPath      = flag.String("out", "", "result JSON path")
		resumePath   = flag.String("resume", "", "reuse successful cases from a previous result JSON")
	)
	flag.Parse()

	if err := loadAIEnv(*envPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		exitf("load environment: %v", err)
	}
	apiKey := strings.TrimSpace(os.Getenv("AI_API_KEY"))
	if apiKey == "" {
		exitf("AI_API_KEY is not configured")
	}
	baseURL := envOr("AI_BASE_URL", "https://api.openai.com/v1")
	model := envOr("AI_MODEL", "gpt-4o-mini")
	timeout, err := time.ParseDuration(envOr("AI_TIMEOUT", "60s"))
	if err != nil {
		exitf("AI_TIMEOUT: %v", err)
	}

	cases, err := readCases(*manifestPath)
	if err != nil {
		exitf("read cases: %v", err)
	}
	if len(cases) != 10 {
		exitf("evaluation requires exactly 10 cases, got %d", len(cases))
	}

	analyzer := openai.New(openai.Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		Timeout: timeout,
	})
	run := evalRun{
		GeneratedAt:   time.Now().UTC(),
		Model:         model,
		PromptVersion: domain.PromptVersion,
		Total:         len(cases),
		ScoringNote:   "Automated exact-fact scoring is a regression signal; release still requires human hallucination review.",
	}
	previous := map[string]evalResult{}
	if *resumePath != "" {
		prior, err := readRun(*resumePath)
		if err != nil {
			exitf("read resume results: %v", err)
		}
		for _, result := range prior.Cases {
			if result.Error == "" && result.Model == model && result.PromptVersion == domain.PromptVersion {
				previous[result.ID] = result
			}
		}
	}

	for index, tc := range cases {
		result, ok := previous[tc.ID]
		if ok {
			fmt.Printf("[%d/%d] %s (reused)\n", index+1, len(cases), tc.ID)
		} else {
			fmt.Printf("[%d/%d] %s\n", index+1, len(cases), tc.ID)
			result = runCase(analyzer, model, *imagesPath, tc)
		}
		run.Cases = append(run.Cases, result)
		if result.AutomatedPass {
			run.AutomatedPassed++
		}
	}
	run.ReleaseGateMet = run.AutomatedPassed >= 8

	if *outPath == "" {
		*outPath = filepath.Join("docs", "evaluation", "results", time.Now().UTC().Format("2006-01-02")+".json")
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		exitf("create result directory: %v", err)
	}
	raw, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		exitf("encode results: %v", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(*outPath, raw, 0o644); err != nil {
		exitf("write results: %v", err)
	}
	fmt.Printf("automated gate: %d/%d pass (release_gate_met=%t)\n", run.AutomatedPassed, run.Total, run.ReleaseGateMet)
	fmt.Printf("results: %s\n", *outPath)
	if !run.ReleaseGateMet {
		os.Exit(1)
	}
}

func runCase(analyzer *openai.Analyzer, model, imagesPath string, tc evalCase) evalResult {
	result := evalResult{
		ID:                 tc.ID,
		Model:              model,
		PromptVersion:      domain.PromptVersion,
		FrameCount:         len(tc.Frames),
		ManualReviewNeeded: true,
	}
	input := domain.AnalysisInput{
		RecordingID:   "eval-" + tc.ID,
		ProjectID:     "vision-evaluation",
		PromptVersion: domain.PromptVersion,
		MetadataJSON:  json.RawMessage(`{"browser":"chrome","os":"darwin","source":"fixed-evaluation"}`),
	}
	for _, name := range tc.Frames {
		data, err := os.ReadFile(filepath.Join(imagesPath, name))
		if err != nil {
			result.Error = err.Error()
			return result
		}
		input.FrameKeys = append(input.FrameKeys, name)
		input.Frames = append(input.Frames, domain.FrameInput{
			StorageKey: name,
			MediaType:  "image/png",
			Data:       data,
		})
	}

	started := time.Now()
	report, err := analyzeWithRetry(analyzer, input)
	result.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Report = report
	result.MatchedFacts, result.MissingFacts, result.ForbiddenMatches, result.AutomatedScore = score(tc, report)
	result.AutomatedPass = result.AutomatedScore >= 8 && len(result.ForbiddenMatches) == 0
	return result
}

func analyzeWithRetry(analyzer *openai.Analyzer, input domain.AnalysisInput) (domain.AnalysisResult, error) {
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		var report domain.AnalysisResult
		report, err = analyzer.Analyze(context.Background(), input)
		if err == nil {
			return report, nil
		}
		if !strings.Contains(err.Error(), "openai status 429") || attempt == 3 {
			return domain.AnalysisResult{}, err
		}
		time.Sleep(time.Duration(1<<attempt) * time.Second)
	}
	return domain.AnalysisResult{}, err
}

func score(tc evalCase, report domain.AnalysisResult) (matched, missing, forbidden []string, score int) {
	text := strings.ToLower(strings.Join(append([]string{report.Title, report.Summary}, report.Steps...), "\n"))
	for _, fact := range tc.RequiredFacts {
		if strings.Contains(text, strings.ToLower(fact)) {
			matched = append(matched, fact)
		} else {
			missing = append(missing, fact)
		}
	}
	for _, claim := range tc.ForbiddenClaims {
		if strings.Contains(text, strings.ToLower(claim)) {
			forbidden = append(forbidden, claim)
		}
	}

	if len(tc.RequiredFacts) > 0 {
		score += 5 * len(matched) / len(tc.RequiredFacts)
	}
	if len(report.Steps) > 0 {
		score += 2
	}
	if strings.Contains(text, "chrome") || strings.Contains(text, "browser") || strings.Contains(text, "darwin") {
		score++
	}
	if strings.TrimSpace(report.Title) != "" && strings.TrimSpace(report.Summary) != "" &&
		len(report.Title) <= 120 && len(report.Summary) <= 800 {
		score += 2
	}
	score -= 3 * len(forbidden)
	if score < 0 {
		score = 0
	}
	return matched, missing, forbidden, score
}

func readCases(path string) ([]evalCase, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []evalCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		return nil, err
	}
	return cases, nil
}

func readRun(path string) (evalRun, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return evalRun{}, err
	}
	var run evalRun
	if err := json.Unmarshal(raw, &run); err != nil {
		return evalRun{}, err
	}
	return run, nil
}

func loadAIEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !strings.HasPrefix(key, "AI_") || os.Getenv(key) != "" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
