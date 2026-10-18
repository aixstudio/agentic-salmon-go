package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aixstudio/agentic-salmon-go/salmon"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	executable, err := os.Executable()
	if err == nil {
		err = run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, executable)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "agentic-salmon:", err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, executable string) error {
	if len(args) > 0 && args[0] == "mcp-server" {
		if len(args) != 1 {
			return fmt.Errorf("mcp-server takes no arguments")
		}
		return salmon.ServeKnowledge(ctx)
	}
	if len(args) > 0 && args[0] == "demo" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("agentic-salmon demo", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fixture := fs.String("fixture", "examples/frozen-salmon-observation.json", "Reviewed perception fixture")
	image := fs.String("input-image", "docs/assets/salmon-pre-cook-input.jpeg", "Fixture-bound input image")
	review := fs.String("review", "pending", "pending, allow-bounded, or stop")
	interactive := fs.Bool("interactive", false, "Prompt at Review and resume the same run")
	selected := fs.String("selected-hypothesis", "fish", "Human-selected hypothesis key")
	storage := fs.String("storage-recollection", "", "Human storage recollection")
	odor := fs.String("odor-observation", "", "Human odor observation")
	outcome := fs.String("outcome", "", "Historical result")
	feedback := fs.String("feedback", "", "Preference feedback")
	reportDir := fs.String("report-directory", "traces", "Local Markdown report directory")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *review != "pending" && *review != "allow-bounded" && *review != "stop" {
		return fmt.Errorf("invalid --review")
	}
	if *interactive && *review != "pending" {
		return fmt.Errorf("--interactive cannot be combined with --review")
	}
	workflow := salmon.NewWorkflow(salmon.FixtureProvider{FixturePath: *fixture, ImagePath: *image}, salmon.MCPKnowledgePort{Executable: executable})
	result, err := workflow.Start(ctx, *outcome, *feedback)
	if err != nil {
		return err
	}
	if *interactive {
		scanner := bufio.NewScanner(in)
		for result.Interrupt != nil {
			display(errOut, result)
			evidence := promptReview(scanner, errOut, result.Interrupt)
			result, err = workflow.Resume(ctx, result.RunID, evidence)
			if err != nil {
				return err
			}
		}
	} else if *review != "pending" {
		evidence := salmon.HumanEvidence{Selected: *selected, Storage: *storage, Odor: *odor, AcceptsUnknowns: salmon.Bool(true), Authorizes: salmon.Bool(true)}
		if *review == "stop" {
			evidence.Concern = true
			evidence.Authorizes = salmon.Bool(false)
		}
		result, err = workflow.Resume(ctx, result.RunID, evidence)
		if err != nil {
			return err
		}
	}
	report, err := writeReport(result, *reportDir)
	if err != nil {
		return err
	}
	payload := struct {
		salmon.RunResult
		LocalReport string `json:"local_report"`
	}{result, report}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
func display(out io.Writer, r salmon.RunResult) {
	fmt.Fprintln(out, "\nReview interrupt for run", r.RunID)
	for _, m := range r.Interrupt.Messages {
		fmt.Fprintln(out, "-", m)
	}
	fmt.Fprintln(out, "Retained unknowns:")
	for _, u := range r.Interrupt.Unknowns {
		fmt.Fprintln(out, "-", u)
	}
	fmt.Fprintln(out, "Hypothesis choices:")
	for i, h := range r.Interrupt.Offered {
		eligibility := "not action eligible"
		if h.Eligible {
			eligibility = "action eligible"
		}
		fmt.Fprintf(out, "%d) %s [%s; %s] (key: %s)\n", i+1, h.Label, h.Status, eligibility, h.ID)
	}
}
func readLine(scanner *bufio.Scanner, out io.Writer, prompt string) (string, bool) {
	fmt.Fprint(out, prompt)
	if !scanner.Scan() {
		return "", false
	}
	return strings.TrimSpace(scanner.Text()), true
}

// EOF and missing authorization fail closed, including piped interactive input.
func promptReview(scanner *bufio.Scanner, out io.Writer, payload *salmon.Interrupt) salmon.HumanEvidence {
	choice, ok := readLine(scanner, out, "Choose a number, hypothesis key, or 'stop': ")
	if !ok || strings.EqualFold(choice, "stop") {
		return salmon.HumanEvidence{Authorizes: salmon.Bool(false)}
	}
	if number, err := strconv.Atoi(choice); err == nil && number > 0 && number <= len(payload.Offered) {
		choice = payload.Offered[number-1].ID
	}
	eligible := false
	for _, h := range payload.Offered {
		if h.ID == choice && h.Eligible {
			eligible = true
		}
	}
	if !eligible {
		return salmon.HumanEvidence{Selected: choice}
	}
	accepts, ok := readLine(scanner, out, "Accept the displayed retained unknowns? [y/N]: ")
	if !ok || !yes(accepts) {
		return salmon.HumanEvidence{Selected: choice, AcceptsUnknowns: salmon.Bool(false), Authorizes: salmon.Bool(false)}
	}
	authorizes, ok := readLine(scanner, out, "Authorize only bounded cited guidance? [y/N]: ")
	if !ok || !yes(authorizes) {
		return salmon.HumanEvidence{Selected: choice, AcceptsUnknowns: salmon.Bool(true), Authorizes: salmon.Bool(false)}
	}
	storage, _ := readLine(scanner, out, "Storage recollection (optional): ")
	odor, _ := readLine(scanner, out, "Odor observation (optional): ")
	return salmon.HumanEvidence{Selected: choice, AcceptsUnknowns: salmon.Bool(true), Authorizes: salmon.Bool(true), Storage: storage, Odor: odor}
}
func yes(s string) bool { return strings.EqualFold(s, "y") || strings.EqualFold(s, "yes") }
func writeReport(r salmon.RunResult, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "agentic-salmon-"+r.RunID+".md")
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Agentic Salmon Go Run\n\n- Run ID: `%s`\n- Status: `%s`\n- Decision: `%s`\n- Total measured node time: %.3f ms\n- Token usage: N/A; reviewed fixture and deterministic contracts\n\n## Measured Node Timing\n\n| Agent | Duration (ms) |\n| --- | ---: |\n", r.RunID, r.Status, r.Decision, r.Telemetry.TotalMS)
	for _, m := range r.Metrics {
		fmt.Fprintf(&b, "| %s | %.3f |\n", m.Stage, m.DurationMS)
	}
	fmt.Fprintf(&b, "\n## Public Trace\n\n```json\n%s\n```\n", data)
	return path, os.WriteFile(path, []byte(b.String()), 0600)
}
