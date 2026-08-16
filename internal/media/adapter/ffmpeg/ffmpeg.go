package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Brohammad/BugSathi/internal/media/port"
)

type Extractor struct {
	Bin       string
	FPS       string // e.g. "0.5" => one frame every 2s
	MaxFrames int
}

func New() *Extractor {
	return &Extractor{Bin: "ffmpeg", FPS: "0.5", MaxFrames: 20}
}

func (e *Extractor) Extract(ctx context.Context, inputPath, outputDir string) (port.Result, error) {
	pattern := filepath.Join(outputDir, "%05d.jpg")
	args := []string{
		"-y", "-i", inputPath,
		"-vf", fmt.Sprintf("fps=%s", e.FPS),
		"-q:v", "3",
		pattern,
	}
	cmd := exec.CommandContext(ctx, e.Bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return port.Result{}, fmt.Errorf("ffmpeg: %w: %s", err, truncate(string(out), 500))
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return port.Result{}, err
	}
	var frames []string
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".jpg") {
			continue
		}
		frames = append(frames, filepath.Join(outputDir, ent.Name()))
	}
	sort.Strings(frames)
	if e.MaxFrames > 0 && len(frames) > e.MaxFrames {
		frames = frames[:e.MaxFrames]
	}
	if len(frames) == 0 {
		return port.Result{}, fmt.Errorf("ffmpeg produced no frames")
	}

	thumb := filepath.Join(outputDir, "thumb.jpg")
	if err := copyFile(frames[0], thumb); err != nil {
		thumb = frames[0]
	}

	durationMS := probeDurationMS(ctx, e.Bin, inputPath)
	return port.Result{FramePaths: frames, ThumbPath: thumb, DurationMS: durationMS}, nil
}

func probeDurationMS(ctx context.Context, ffmpegBin, input string) int64 {
	// Prefer ffprobe if present; else skip.
	probe := strings.Replace(ffmpegBin, "ffmpeg", "ffprobe", 1)
	cmd := exec.CommandContext(ctx, probe,
		"-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", input,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	sec, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return int64(sec * 1000)
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
