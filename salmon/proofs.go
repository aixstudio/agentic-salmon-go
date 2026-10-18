package salmon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const HistoricalOutcome = "Historical case record: 390°F for 14 minutes; wonderful and fresh-tasting. Not a new cooking trial or proof of safety."
const HistoricalFeedback = "Historical case preference: moderately spicy result preferred."

var validRunID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type savedRun struct {
	Schema     int             `json:"schema"`
	Result     RunResult       `json:"result"`
	Perception *Perception     `json:"perception"`
	Connected  *ConnectedEvent `json:"connected"`
	Reason     *ReasonResult   `json:"reason"`
	Review     *ReviewResult   `json:"review"`
	Action     *ActionResult   `json:"action"`
	Outcome    string          `json:"outcome"`
	Feedback   string          `json:"feedback"`
}
type ProofStore struct {
	Directory, RunID, Database, Log string
	db                              *sql.DB
}

func OpenProofStore(directory, id string, reserve bool) (*ProofStore, error) {
	if !validRunID.MatchString(id) {
		return nil, fmt.Errorf("run ID must contain 1–64 letters, digits, underscores or hyphens")
	}
	s := &ProofStore{Directory: directory, RunID: id, Database: filepath.Join(directory, id+".sqlite3"), Log: filepath.Join(directory, id+".log")}
	if reserve {
		if err := os.MkdirAll(directory, 0700); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(s.Database, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			if os.IsExist(err) {
				return nil, fmt.Errorf("run ID already exists; continue its next proof or use a new run ID")
			}
			return nil, err
		}
		if err = f.Close(); err != nil {
			return nil, err
		}
	} else if _, err := os.Stat(s.Database); err != nil {
		return nil, fmt.Errorf("unknown run ID; run Proof-1 first")
	}
	db, err := sql.Open("sqlite", s.Database)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s.db = db
	if _, err = db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, err
	}
	if reserve {
		_, err = db.Exec("CREATE TABLE proof_run (id INTEGER PRIMARY KEY CHECK(id=1), phase TEXT NOT NULL, payload BLOB); INSERT INTO proof_run VALUES (1, 'starting', NULL)")
		if err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}
func (s *ProofStore) Close() error { return s.db.Close() }
func (s *ProofStore) load(expected string) (savedRun, error) {
	var phase string
	var data []byte
	var out savedRun
	if err := s.db.QueryRow("SELECT phase,payload FROM proof_run WHERE id=1").Scan(&phase, &data); err != nil {
		return out, fmt.Errorf("checkpoint is not a supported proof database: %w", err)
	}
	if phase != expected {
		return out, fmt.Errorf("expected %s, found %s; no stage was rerun", expected, phase)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	if out.Schema != 1 || out.Result.RunID != s.RunID || out.Perception == nil || out.Connected == nil || out.Reason == nil {
		return out, fmt.Errorf("invalid saved proof artifacts")
	}
	return out, nil
}
func (s *ProofStore) Read(expected string) (RunResult, error) {
	c, err := s.load(expected)
	return c.Result, err
}
func (s *ProofStore) claim(expected, active string) error {
	result, err := s.db.Exec("UPDATE proof_run SET phase=? WHERE id=1 AND phase=?", active, expected)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("another process already claimed this proof; no stage was rerun")
	}
	return nil
}
func (s *ProofStore) record(value any) error {
	f, err := os.OpenFile(s.Log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = json.NewEncoder(f).Encode(value); err != nil {
		return err
	}
	return f.Sync()
}
func (s *ProofStore) save(phase string, w *Workflow, result RunResult) error {
	state := w.runs[s.RunID]
	saved := savedRun{1, result, state.artifacts.Perception, state.artifacts.Connected, state.artifacts.Reason, state.artifacts.Review, state.artifacts.Action, state.outcome, state.feedback}
	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	if _, err = s.db.Exec("UPDATE proof_run SET phase=?,payload=? WHERE id=1", phase, data); err != nil {
		return err
	}
	return s.record(struct {
		At     string    `json:"at"`
		Phase  string    `json:"phase"`
		Result RunResult `json:"result"`
	}{time.Now().UTC().Format(time.RFC3339Nano), phase, result})
}

// RunProof claims each operation before execution. A crashed active claim is
// blocked from automatic retry; this local demo does not promise distributed exactly-once effects.
func RunProof(ctx context.Context, command string, s *ProofStore, decision string, provider VisionProvider, knowledge KnowledgePort) (result RunResult, err error) {
	var prior savedRun
	if command == "proof-2" || command == "proof-3" {
		expected, active := "paused", "reviewing"
		if command == "proof-3" {
			expected, active = "adjudicated", "executing"
		} else if decision != "APPROVE" && decision != "STOP" && decision != "" {
			return result, fmt.Errorf("review requires explicit APPROVE or STOP")
		}
		prior, err = s.load(expected)
		if err != nil {
			return result, err
		}
		if err = s.claim(expected, active); err != nil {
			return result, err
		}
	} else if command != "proof-1" {
		return result, fmt.Errorf("unknown proof")
	} else if err = s.claim("starting", "preparing"); err != nil {
		return result, err
	}
	defer func() {
		if err != nil {
			_ = s.record(map[string]string{"phase": "failed", "error": err.Error()})
			_, _ = s.db.Exec("UPDATE proof_run SET phase='failed' WHERE id=1")
			err = fmt.Errorf("execution failed; details: %s", s.Log)
		}
	}()
	if provider == nil {
		provider = FixtureProvider{"examples/frozen-salmon-observation.json", "docs/assets/salmon-pre-cook-input.jpeg"}
	}
	if knowledge == nil {
		exe, e := os.Executable()
		if e != nil {
			return result, e
		}
		details, e := os.OpenFile(s.Log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			return result, e
		}
		defer details.Close()
		knowledge = MCPKnowledgePort{Executable: exe, Stderr: details}
	}
	w := NewWorkflow(provider, knowledge)
	if command != "proof-1" {
		w.runs[s.RunID] = &runState{result: prior.Result, artifacts: TransitionArtifacts{prior.Perception, prior.Connected, prior.Reason, prior.Review, prior.Action}, outcome: prior.Outcome, feedback: prior.Feedback}
	}
	phase := "paused"
	switch command {
	case "proof-1":
		result, err = w.StartWithID(ctx, s.RunID, HistoricalOutcome, HistoricalFeedback)
	case "proof-2":
		approved := decision == "APPROVE"
		selected := ""
		if approved {
			selected = "fish"
		}
		result, err = w.Adjudicate(ctx, s.RunID, HumanEvidence{Selected: selected, AcceptsUnknowns: Bool(approved), Authorizes: Bool(approved)})
		phase = "adjudicated"
		if !approved {
			phase = "stopped"
		}
	case "proof-3":
		result, err = w.Execute(ctx, s.RunID)
		phase = "completed"
	}
	if err != nil {
		return result, err
	}
	status, count := "interrupted", 3
	if command == "proof-2" {
		status, count = "adjudicated", 4
		if phase == "stopped" {
			status = "stopped"
		}
	} else if command == "proof-3" {
		status, count = "completed", 6
	}
	if result.Status != status || len(result.Events) != count {
		return result, fmt.Errorf("proof contract failed")
	}
	err = s.save(phase, w, result)
	return result, err
}
func ReviewPacket(result RunResult) (string, error) {
	if len(result.Events) < 3 {
		return "", fmt.Errorf("missing saved evidence")
	}
	data, err := json.Marshal(result.Events[2].Evidence)
	if err != nil {
		return "", err
	}
	var reason ReasonResult
	if err = json.Unmarshal(data, &reason); err != nil {
		return "", err
	}
	supported := false
	for _, h := range reason.Hypotheses {
		if h.ID == "fish" && h.Status == Supported {
			supported = true
		}
	}
	if !supported {
		return "", fmt.Errorf("saved evidence does not support the deterministic fish review contract")
	}
	return fmt.Sprintf("HITL review — run %s (saved Proof-1 evidence)\nReview: visible portions and skin-like surface support fish as a working hypothesis; species is unverified.\nRetain ALL unknowns: %s.\nAuthority requested: bounded thermometer guidance with a retrieved citation; no safety or identity certification.\nAPPROVE selects fish, accepts these unknowns, and authorizes only that guidance.\nWatch out: do not supply guessed observations. STOP/Enter ends without action.", result.RunID, strings.Join(reason.Unknowns, "; ")), nil
}
func ProofSummary(command string, r RunResult, s *ProofStore) string {
	lines := []string{fmt.Sprintf("%s | run %s | %s", strings.ToUpper(command), r.RunID, strings.ToUpper(r.Status))}
	next := ""
	switch {
	case command == "proof-1":
		lines = append(lines, "Perceive → Connect → Reason completed; real RAG/MCP evidence saved.", strings.Join(reviewLines(r), "\n"), "Watch out: retrieved knowledge is not permission. Act/Learn have NOT run.")
		next = "proof-2"
	case r.Status == "stopped":
		lines = append(lines, "Human declined. Act/Learn have NOT run.", "TEST END — STOPPED. No further proof may execute this run.")
	case command == "proof-2":
		lines = append(lines, "HITL adjudication saved: ALLOW_BOUNDED; all unknowns retained.", "Watch out: approval is recorded; Act/Learn have NOT run.")
		next = "proof-3"
	default:
		lines = append(lines, "Act released human-authorized, cited guidance; Learn recorded the fixed historical case outcome.", "Watch out: identity/safety remain unverified; historical outcome changes no policy.", "TEST END — PASS. All six agents executed once; no further action required.")
	}
	if next != "" {
		cmd := fmt.Sprintf("Next: ./bin/agentic-salmon %s --run-id %s", next, r.RunID)
		if s.Directory != "log" {
			cmd += " --log-directory " + s.Directory
		}
		lines = append(lines, cmd)
	}
	lines = append(lines, "Details: "+s.Log)
	return strings.Join(lines, "\n")
}

func reviewLines(r RunResult) []string {
	packet, err := ReviewPacket(r)
	if err != nil {
		return []string{"Review packet unavailable; inspect the run log."}
	}
	return strings.Split(packet, "\n")[1:4]
}
