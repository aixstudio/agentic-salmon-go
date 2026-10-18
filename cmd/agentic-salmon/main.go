package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/aixstudio/agentic-salmon-go/salmon"
	"io"
	"os"
	"os/signal"
	"strings"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	exe, err := os.Executable()
	if err == nil {
		err = run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, exe)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Proof blocked:", err)
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
	if len(args) == 0 {
		return fmt.Errorf("run proof-1, proof-2, then proof-3 with --run-id")
	}
	command := args[0]
	if command != "proof-1" && command != "proof-2" && command != "proof-3" {
		return fmt.Errorf("expected proof-1, proof-2 or proof-3")
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	id := fs.String("run-id", "", "Same run ID for all three proofs")
	dir := fs.String("log-directory", "log", "Local checkpoints and detailed run logs")
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 || *id == "" {
		return fmt.Errorf("--run-id is required; no positional arguments accepted")
	}
	store, err := salmon.OpenProofStore(*dir, *id, command == "proof-1")
	if err != nil {
		return err
	}
	defer store.Close()
	decision := ""
	if command == "proof-2" {
		result, err := store.Read("paused")
		if err != nil {
			return err
		}
		packet, err := salmon.ReviewPacket(result)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, packet)
		fmt.Fprint(out, "Type APPROVE to accept this contract, or STOP/Enter to end: ")
		scanner := bufio.NewScanner(in)
		if scanner.Scan() {
			decision = strings.ToUpper(strings.TrimSpace(scanner.Text()))
		} else {
			decision = "STOP"
		}
		if decision != "APPROVE" && decision != "STOP" && decision != "" {
			return fmt.Errorf("input not accepted; run remains paused; rerun Proof-2 and enter APPROVE or STOP")
		}
	}
	details, err := os.OpenFile(store.Log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer details.Close()
	result, err := salmon.RunProof(ctx, command, store, decision, nil, salmon.MCPKnowledgePort{Executable: executable, Stderr: details})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, salmon.ProofSummary(command, result, store))
	return nil
}
