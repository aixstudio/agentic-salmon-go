package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aixstudio/agentic-salmon-go/salmon"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "mcp-server" {
		if err := salmon.ServeKnowledge(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[1] == "cli-proof" {
		if err := run(context.Background(), os.Args[2:], os.Stdin, os.Stdout, os.Stderr, os.Args[0]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
func invoke(t *testing.T, dir, id, proof, input string, wantError bool) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "cli-proof", proof, "--run-id", id, "--log-directory", dir)
	cmd.Stdin = strings.NewReader(input)
	data, err := cmd.CombinedOutput()
	if (err != nil) != wantError {
		t.Fatalf("%s: %v %s", proof, err, data)
	}
	return string(data)
}
func readResult(t *testing.T, dir, id, phase string) salmon.RunResult {
	t.Helper()
	store, err := salmon.OpenProofStore(dir, id, false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	r, err := store.Read(phase)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestThreeProofsAcrossProcesses(t *testing.T) {
	dir := t.TempDir()
	id := "cli-001"
	out := invoke(t, dir, id, "proof-1", "", false)
	if !strings.Contains(out, "Act/Learn have NOT run") || strings.Contains(out, "\"events\"") {
		t.Fatal("terminal output is not bounded guidance")
	}
	first := readResult(t, dir, id, "paused")
	if len(first.Events) != 3 || first.Guidance != nil {
		t.Fatal("Proof-1 released action")
	}
	invoke(t, dir, id, "proof-3", "", true)
	out = invoke(t, dir, id, "proof-2", "guess\n", true)
	if !strings.Contains(out, "remains paused") {
		t.Fatal("invalid answer advanced")
	}
	out = invoke(t, dir, id, "proof-2", "APPROVE\n", false)
	if strings.Contains(out, "Storage recollection") || strings.Contains(out, "Odor observation") {
		t.Fatal("free text prompts remain")
	}
	second := readResult(t, dir, id, "adjudicated")
	if len(second.Events) != 4 || second.Guidance != nil || second.RunID != first.RunID {
		t.Fatal("Proof-2 did not stop before action")
	}
	a, _ := json.Marshal(first.Events)
	b, _ := json.Marshal(second.Events[:3])
	if !bytes.Equal(a, b) {
		t.Fatal("evidence was rerun")
	}
	out = invoke(t, dir, id, "proof-3", "", false)
	if !strings.Contains(out, "TEST END — PASS") {
		t.Fatal("missing test-end acknowledgement")
	}
	third := readResult(t, dir, id, "completed")
	if len(third.Events) != 6 || third.Guidance == nil {
		t.Fatal("Proof-3 did not complete")
	}
	invoke(t, dir, id, "proof-3", "", true)
	invoke(t, dir, id, "proof-1", "", true)
	data, err := os.ReadFile(filepath.Join(dir, id+".log"))
	if err != nil || !bytes.Contains(data, []byte("citation_chunk_ids")) {
		t.Fatal("missing detailed log")
	}
}
func TestReviewStopAndEOF(t *testing.T) {
	for _, input := range []string{"STOP\n", "", "\n"} {
		dir := t.TempDir()
		invoke(t, dir, "stop-001", "proof-1", "", false)
		invoke(t, dir, "stop-001", "proof-2", input, false)
		r := readResult(t, dir, "stop-001", "stopped")
		if r.Guidance != nil || len(r.Events) != 4 {
			t.Fatal("stop released action")
		}
		invoke(t, dir, "stop-001", "proof-3", "", true)
	}
}
func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{}, {"demo"}, {"proof-1"}, {"proof-1", "--run-id", "../bad"}, {"proof-1", "--run-id", "x", "--interactive"}} {
		if err := run(context.Background(), args, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, "unused"); err == nil {
			t.Fatal("invalid args accepted")
		}
	}
}
