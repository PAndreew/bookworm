package chunk

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

//go:embed sentences.py
var sentenceScript string

var segmentSlots = make(chan struct{}, 2)

// Sentences runs pinned pySBD once per layout build, batching its chapters.
// Python indices and Go rune indices agree even for astral Unicode characters.
// Fail closed: never publish a replacement with guessed or damaged offsets.
func Sentences(ctx context.Context, texts []string) ([][]CardSpan, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	select {
	case segmentSlots <- struct{}{}:
		defer func() { <-segmentSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	input, err := json.Marshal(texts)
	if err != nil {
		return nil, err
	}
	python := os.Getenv("PYSBD_PYTHON")
	if python == "" {
		python = "python3"
	}
	cmd := exec.CommandContext(ctx, python, "-c", sentenceScript)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pySBD segmentation failed: %w: %s", err, stderr.String())
	}
	var result [][]CardSpan
	if err = json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("pySBD offsets: %w", err)
	}
	if len(result) != len(texts) {
		return nil, fmt.Errorf("pySBD returned wrong chapter count")
	}
	for i, spans := range result {
		r, previous := []rune(texts[i]), 0
		for _, span := range spans {
			if span.Start < previous || span.End <= span.Start || span.End > len(r) {
				return nil, fmt.Errorf("pySBD invalid span in chapter %d", i)
			}
			if strings.TrimSpace(string(r[previous:span.Start])) != "" {
				return nil, fmt.Errorf("pySBD skipped text in chapter %d", i)
			}
			previous = span.End
		}
		if strings.TrimSpace(string(r[previous:])) != "" {
			return nil, fmt.Errorf("pySBD lost trailing text in chapter %d", i)
		}
	}
	return result, nil
}
