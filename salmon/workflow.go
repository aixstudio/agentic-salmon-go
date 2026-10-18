package salmon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type TransitionArtifacts struct {
	Perception *Perception
	Connected  *ConnectedEvent
	Reason     *ReasonResult
	Review     *ReviewResult
	Action     *ActionResult
}
type GuardPolicy struct{}

func (GuardPolicy) Require(stage Stage, a TransitionArtifacts) error {
	allowed := false
	switch stage {
	case PerceiveStage:
		allowed = true
	case ConnectStage:
		allowed = a.Perception != nil
	case ReasonStage:
		allowed = a.Connected != nil
	case ReviewStage:
		allowed = a.Reason != nil
	case ActStage:
		allowed = a.Review != nil && a.Review.Decision == AllowBounded
	case LearnStage:
		allowed = a.Action != nil
	}
	if !allowed {
		return fmt.Errorf("Guard blocked transition to %s", stage)
	}
	return nil
}

type runState struct {
	result            RunResult
	artifacts         TransitionArtifacts
	outcome, feedback string
}

// Workflow keeps in-memory checkpoints for this process. A mutex serializes
// starts and resumes; completed/stopped runs cannot be resumed or replay Act.
type Workflow struct {
	perceive PerceiveAgent
	reason   ReasonAgent
	guard    GuardPolicy
	runs     map[string]*runState
	mu       sync.Mutex
}

func NewWorkflow(provider VisionProvider, knowledge KnowledgePort) *Workflow {
	return &Workflow{perceive: PerceiveAgent{provider}, reason: ReasonAgent{knowledge}, runs: map[string]*runState{}}
}
func newRunID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func (s *runState) record(stage Stage, started time.Time, summary string, evidence any) {
	ended := time.Now()
	s.result.Events = append(s.result.Events, Event{len(s.result.Events) + 1, stage, summary, evidence})
	metric := Metric{stage, started.UTC().Format(time.RFC3339Nano), ended.UTC().Format(time.RFC3339Nano), float64(ended.Sub(started).Microseconds()) / 1000}
	s.result.Metrics = append(s.result.Metrics, metric)
	s.result.Telemetry.TotalMS += metric.DurationMS
}
func (s *runState) interrupt(messages []string) {
	r := s.artifacts.Reason
	offered := []OfferedHypothesis{}
	for _, h := range r.Hypotheses {
		offered = append(offered, OfferedHypothesis{h.ID, h.Name, h.Status, h.Status == Supported || h.Status == Plausible})
	}
	s.result.Status = "interrupted"
	s.result.Interrupt = &Interrupt{Question: "Select a working hypothesis and authorize only bounded guidance.", Messages: append([]string{}, messages...), RequiredFields: []string{"selected_hypothesis", "accepts_retained_unknowns", "authorizes_bounded_guidance"}, Unknowns: append([]string{}, r.Unknowns...), Offered: offered, AllowedDecisions: []string{"ASK", "STOP", "ALLOW_BOUNDED"}}
}

// snapshot prevents API callers from mutating checkpoint authority or artifacts.
func snapshot(r RunResult) RunResult {
	data, _ := json.Marshal(r)
	var out RunResult
	_ = json.Unmarshal(data, &out)
	return out
}
func (w *Workflow) Start(ctx context.Context, outcome, feedback string) (RunResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	id, err := newRunID()
	if err != nil {
		return RunResult{}, err
	}
	s := &runState{outcome: outcome, feedback: feedback, result: RunResult{RunID: id, Status: "running", Decision: Ask, Events: []Event{}, Metrics: []Metric{}, Telemetry: Telemetry{Note: "N/A; reviewed perception fixture and deterministic agent contracts"}}}
	started := time.Now()
	p, err := w.perceive.Run()
	if err != nil {
		return RunResult{}, err
	}
	s.artifacts.Perception = &p
	s.record(PerceiveStage, started, "Separated sourced observations from hypotheses and unknowns.", p)
	if err = w.guard.Require(ConnectStage, s.artifacts); err != nil {
		return RunResult{}, err
	}
	started = time.Now()
	c, err := (ConnectAgent{}).Run(p)
	if err != nil {
		return RunResult{}, err
	}
	s.artifacts.Connected = &c
	s.record(ConnectStage, started, "Resolved mentions and assembled an ontology-valid event.", c)
	if err = w.guard.Require(ReasonStage, s.artifacts); err != nil {
		return RunResult{}, err
	}
	started = time.Now()
	r, err := w.reason.Run(ctx, p, c)
	if err != nil {
		return RunResult{}, err
	}
	s.artifacts.Reason = &r
	s.record(ReasonStage, started, "Called ranked RAG through the configured knowledge capability and produced a cited proposal.", r)
	if err = w.guard.Require(ReviewStage, s.artifacts); err != nil {
		return RunResult{}, err
	}
	s.interrupt(nil)
	w.runs[id] = s
	return snapshot(s.result), nil
}
func (w *Workflow) Resume(ctx context.Context, id string, e HumanEvidence) (RunResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	s, ok := w.runs[id]
	if !ok {
		return RunResult{}, fmt.Errorf("unknown run ID")
	}
	if s.result.Status != "interrupted" {
		return RunResult{}, fmt.Errorf("run is not awaiting Review")
	}
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	if err := w.guard.Require(ReviewStage, s.artifacts); err != nil {
		return RunResult{}, err
	}
	started := time.Now()
	v := (ReviewAgent{}).Run(*s.artifacts.Reason, e)
	s.artifacts.Review = &v
	s.result.Decision = v.Decision
	s.record(ReviewStage, started, "Human review returned "+string(v.Decision)+".", struct {
		Human  HumanEvidence `json:"human_evidence"`
		Review ReviewResult  `json:"review"`
	}{e, v})
	if v.Decision == Ask {
		s.interrupt(v.Reasons)
		return snapshot(s.result), nil
	}
	s.result.Interrupt = nil
	if v.Decision == Stop {
		s.result.Status = "stopped"
		return snapshot(s.result), nil
	}
	if err := w.guard.Require(ActStage, s.artifacts); err != nil {
		return RunResult{}, err
	}
	started = time.Now()
	a, err := (ActAgent{}).Run(*s.artifacts.Reason, v)
	if err != nil {
		s.result.Status = "failed"
		return snapshot(s.result), err
	}
	s.artifacts.Action = &a
	s.result.Guidance = &a.Guidance
	s.record(ActStage, started, "Released only the human-authorized bounded intervention.", a)
	if err = w.guard.Require(LearnStage, s.artifacts); err != nil {
		return RunResult{}, err
	}
	started = time.Now()
	l, err := (LearnAgent{}).Run(LearnInput{a, s.outcome, s.feedback})
	if err != nil {
		s.result.Status = "failed"
		return snapshot(s.result), err
	}
	s.record(LearnStage, started, "Recorded outcome evidence without changing policy.", l)
	s.result.Status = "completed"
	return snapshot(s.result), nil
}
