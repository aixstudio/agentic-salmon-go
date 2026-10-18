package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aixstudio/agentic-salmon-go/salmon"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "mcp-server" {
		if err := salmon.ServeKnowledge(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestCLIPaths(t *testing.T) {
	cases := []struct {
		name, input, status string
		extra               []string
		events              int
	}{
		{"pending", "", "interrupted", nil, 3},
		{"approved", "", "completed", []string{"--review", "allow-bounded"}, 6},
		{"stop", "", "stopped", []string{"--review", "stop"}, 4},
		{"unknown", "", "interrupted", []string{"--review", "allow-bounded", "--selected-hypothesis", "korean_spiced_soup_food"}, 4},
		{"interactive number", "2\ny\ny\nfreezer recollection\nseasoning odor\n", "completed", []string{"--interactive"}, 6},
		{"interactive retry", "chicken\n1\nfish\ny\ny\n\n\n", "completed", []string{"--interactive"}, 8},
		{"interactive eof", "", "stopped", []string{"--interactive"}, 4},
		{"interactive refusal", "2\nn\n", "stopped", []string{"--interactive"}, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := []string{"demo", "--fixture", "../../examples/frozen-salmon-observation.json", "--input-image", "../../docs/assets/salmon-pre-cook-input.jpeg", "--report-directory", t.TempDir()}
			args = append(args, c.extra...)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			if err = run(context.Background(), args, strings.NewReader(c.input), &out, &stderr, executable); err != nil {
				t.Fatal(err)
			}
			var result struct {
				salmon.RunResult
				Report string `json:"local_report"`
			}
			if err = json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("stdout is not clean JSON: %v", err)
			}
			if result.Status != c.status || len(result.Events) != c.events {
				t.Fatalf("unexpected CLI result: %s events=%d; stderr=%s", result.Status, len(result.Events), stderr.String())
			}
			if _, err = os.Stat(result.Report); err != nil {
				t.Fatal("missing report", err)
			}
		})
	}
}
func TestPromptRejectsSelectionBeforeAuthorityQuestions(t *testing.T) {
	p := &salmon.Interrupt{Offered: []salmon.OfferedHypothesis{{ID: "unknown", Status: salmon.Unknown}, {ID: "fish", Eligible: true}}}
	for _, input := range []string{"1\n", "chicken\n"} {
		var out bytes.Buffer
		e := promptReview(bufio.NewScanner(strings.NewReader(input)), &out, p)
		if e.Authorizes != nil || strings.Contains(out.String(), "Authorize") {
			t.Fatal("invalid selection consumed authority input")
		}
	}
}
func TestCLIRejectsConflictingOrInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"demo", "--interactive", "--review", "stop"}, {"demo", "--review", "typo"}, {"unknown"}, {"mcp-server", "extra"}} {
		if err := run(context.Background(), args, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, "unused"); err == nil {
			t.Fatal("invalid CLI arguments accepted")
		}
	}
}
