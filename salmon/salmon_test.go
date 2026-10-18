package salmon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The test executable acts as a real SDK stdio server in child processes.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "mcp-server" {
		if err := ServeKnowledge(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func fixture() FixtureProvider {
	return FixtureProvider{"../examples/frozen-salmon-observation.json", "../docs/assets/salmon-pre-cook-input.jpeg"}
}

type staticProvider struct{ p Perception }

func (p staticProvider) Perceive() (Perception, error) { return p.p, nil }

type localKnowledge struct{}

func (localKnowledge) Search(ctx context.Context, q string, k int) (KnowledgeResult, error) {
	r, err := NewRetriever()
	if err != nil {
		return KnowledgeResult{}, err
	}
	chunks, err := r.Search(q, k)
	return KnowledgeResult{Query: q, Model: RetrievalModel, Chunks: chunks}, err
}

type emptyKnowledge struct{}

func (emptyKnowledge) Search(context.Context, string, int) (KnowledgeResult, error) {
	return KnowledgeResult{}, nil
}
func perception(t *testing.T) Perception {
	t.Helper()
	p, err := (PerceiveAgent{fixture()}).Run()
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func reasoning(t *testing.T) ReasonResult {
	t.Helper()
	p := perception(t)
	c, err := (ConnectAgent{}).Run(p)
	if err != nil {
		t.Fatal(err)
	}
	r, err := (ReasonAgent{localKnowledge{}}).Run(context.Background(), p, c)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func authorized() HumanEvidence {
	return HumanEvidence{Selected: "fish", AcceptsUnknowns: Bool(true), Authorizes: Bool(true)}
}

func TestPerceiveContract(t *testing.T) {
	for _, name := range []string{"provenance", "observations", "unknowns", "duplicate observation", "duplicate candidate", "unknown support", "unknown source"} {
		t.Run(name, func(t *testing.T) {
			p := perception(t)
			switch name {
			case "provenance":
				p.Provenance = " "
			case "observations":
				p.Observations = nil
			case "unknowns":
				p.Unknowns = nil
			case "duplicate observation":
				p.Observations = append(p.Observations, p.Observations[0])
			case "duplicate candidate":
				p.Candidates = append(p.Candidates, p.Candidates[0])
			case "unknown support":
				p.Candidates[0].SupportingIDs = []string{"absent"}
			case "unknown source":
				p.Candidates[0].Source = "injected"
			}
			if _, err := (PerceiveAgent{staticProvider{p}}).Run(); err == nil {
				t.Fatal("invalid perception accepted")
			}
		})
	}
}
func TestFixtureRejectsChangedImage(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image.jpeg")
	if err := os.WriteFile(image, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (FixtureProvider{fixture().FixturePath, image}).Perceive(); err == nil {
		t.Fatal("changed image accepted")
	}
}
func TestFixtureRejectsUnknownFields(t *testing.T) {
	for _, key := range []string{"confidence", "canonical_id", "hidden_reasoning"} {
		t.Run(key, func(t *testing.T) {
			raw, err := os.ReadFile(fixture().FixturePath)
			if err != nil {
				t.Fatal(err)
			}
			var data map[string]any
			if err = json.Unmarshal(raw, &data); err != nil {
				t.Fatal(err)
			}
			if key == "canonical_id" {
				data["entity_mentions"].([]any)[0].(map[string]any)[key] = "upstream"
			} else {
				data[key] = "untrusted"
			}
			raw, err = json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "fixture.json")
			if err = os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := (FixtureProvider{path, fixture().ImagePath}).Perceive(); err == nil {
				t.Fatal("unknown fixture field accepted")
			}
		})
	}
}
func TestConnectResolvesAndValidatesOntology(t *testing.T) {
	p := perception(t)
	c, err := (ConnectAgent{}).Run(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entities) != 3 || len(c.Relationships) != 2 || c.Ontology.Version != "1.0.0" {
		t.Fatalf("unexpected connected event: %+v", c)
	}
	for _, r := range c.Relationships {
		if r.Subject != "food_portions_1" {
			t.Fatal("food mentions were not resolved")
		}
	}
	for _, name := range []string{"reversed relationship", "unknown endpoint", "duplicate mention", "unknown kind"} {
		t.Run(name, func(t *testing.T) {
			p := perception(t)
			switch name {
			case "reversed relationship":
				p.Relationships[0].Subject, p.Relationships[0].Object = p.Relationships[0].Object, p.Relationships[0].Subject
			case "unknown endpoint":
				p.Relationships[0].Object = "absent"
			case "duplicate mention":
				p.Entities = append(p.Entities, p.Entities[0])
			case "unknown kind":
				p.Entities[0].Kind = "unrecognized"
			}
			if _, err := (ConnectAgent{}).Run(p); err == nil {
				t.Fatal("ontology violation accepted")
			}
		})
	}
}
func TestReasonSeparatesHypothesesAndUsesRetrieval(t *testing.T) {
	r := reasoning(t)
	statuses := []HypothesisStatus{Unknown, Supported, Plausible}
	for i, h := range r.Hypotheses {
		if h.Status != statuses[i] {
			t.Fatalf("wrong hypothesis status: %+v", h)
		}
	}
	if len(r.Knowledge.Chunks) != 3 || len(r.Unknowns) != 5 || !strings.Contains(r.Query, "air-fryer") {
		t.Fatal("Reason did not augment evidence with ranked knowledge")
	}
	p := perception(t)
	c, _ := (ConnectAgent{}).Run(p)
	if _, err := (ReasonAgent{emptyKnowledge{}}).Run(context.Background(), p, c); err == nil {
		t.Fatal("empty RAG accepted")
	}
}
func TestReviewAuthority(t *testing.T) {
	cases := []struct {
		name string
		e    HumanEvidence
		want Decision
	}{
		{"missing", HumanEvidence{}, Ask}, {"approved", authorized(), AllowBounded},
		{"unknown", HumanEvidence{Selected: "korean_spiced_soup_food", AcceptsUnknowns: Bool(true), Authorizes: Bool(true)}, Ask},
		{"unoffered", HumanEvidence{Selected: "chicken", AcceptsUnknowns: Bool(true), Authorizes: Bool(true)}, Ask},
		{"refused", HumanEvidence{Authorizes: Bool(false)}, Stop},
		{"concern", HumanEvidence{Concern: true}, Stop},
		{"no uncertainty acceptance", HumanEvidence{Selected: "fish", Authorizes: Bool(true)}, Ask},
		{"no authorization", HumanEvidence{Selected: "fish", AcceptsUnknowns: Bool(true)}, Ask},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := (ReviewAgent{}).Run(reasoning(t), c.e); got.Decision != c.want {
				t.Fatalf("got %s, want %s", got.Decision, c.want)
			}
		})
	}
	e := authorized()
	e.Odor = "seasoning odor"
	e.Storage = "freezer recollection"
	v := (ReviewAgent{}).Run(reasoning(t), e)
	if contains(v.Resolved, "odor after opening") || !contains(v.Unknowns, "odor after opening") || !contains(v.Unknowns, "storage and cold-chain history") || !contains(v.Unknowns, "verified food identity") {
		t.Fatal("human recollection erased unresolved evidence")
	}
}
func TestActAndLearnContracts(t *testing.T) {
	r := reasoning(t)
	v := (ReviewAgent{}).Run(r, authorized())
	a, err := (ActAgent{}).Run(r, v)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.CitationIDs) == 0 || !strings.Contains(a.Guidance, "Identity remains unverified") || !strings.Contains(a.Guidance, "fish hypothesis") {
		t.Fatal("action lost evidence bounds")
	}
	for _, name := range []string{"bypass", "unoffered", "unknown", "no citation", "tampered citation"} {
		t.Run(name, func(t *testing.T) {
			r := reasoning(t)
			v := (ReviewAgent{}).Run(r, authorized())
			switch name {
			case "bypass":
				v.Decision = Ask
			case "unoffered":
				v.Selected = "chicken"
			case "unknown":
				v.Selected = "korean_spiced_soup_food"
			case "no citation":
				r.Knowledge.Chunks = nil
			case "tampered citation":
				for i := range r.Knowledge.Chunks {
					r.Knowledge.Chunks[i].SHA256 = "tampered"
				}
			}
			if _, err := (ActAgent{}).Run(r, v); err == nil {
				t.Fatal("invalid action authorized")
			}
		})
	}
	l, err := (LearnAgent{}).Run(LearnInput{a, "historical success", "spicy preference"})
	if err != nil || !strings.HasPrefix(l.PolicyEffect, "none;") {
		t.Fatal("learning changed policy")
	}
	if _, err := (LearnAgent{}).Run(LearnInput{}); err == nil {
		t.Fatal("learning without cited action accepted")
	}
}
func TestGuardRejectsMissingArtifacts(t *testing.T) {
	for _, stage := range []Stage{ConnectStage, ReasonStage, ReviewStage, ActStage, LearnStage, "unknown"} {
		if err := (GuardPolicy{}).Require(stage, TransitionArtifacts{}); err == nil {
			t.Fatalf("Guard admitted %s", stage)
		}
	}
}
func TestRetrievalRanksQueryAndResolvesSource(t *testing.T) {
	r, err := NewRetriever()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ q, id string }{{"thermometer 145 fish internal temperature", "usda-air-fryer-temperature"}, {"frozen seafood spoil cold chain storage", "fda-seafood-storage-unknown"}, {"overfill basket hot air circulation batches", "usda-air-fryer-circulation"}} {
		chunks, err := r.Search(c.q, 3)
		if err != nil || len(chunks) == 0 || chunks[0].ID != c.id {
			t.Fatalf("ranking for %q: %+v, %v", c.q, chunks, err)
		}
		for i, chunk := range chunks {
			if chunk.SHA256 != digest([]byte(chunk.Text)) || chunk.URL == "" {
				t.Fatal("missing source integrity")
			}
			if i > 0 && chunks[i-1].Score < chunk.Score {
				t.Fatal("results unranked")
			}
		}
	}
	for _, k := range []int{-1, 0, 6} {
		if _, err := r.Search("fish", k); err == nil {
			t.Fatal("unbounded top_k")
		}
	}
	if _, err := r.Search(" ", 3); err == nil {
		t.Fatal("empty query accepted")
	}
	chunks, err := r.Search("zzzzunmatched", 3)
	if err != nil || len(chunks) != 0 {
		t.Fatal("unrelated query returned fabricated evidence")
	}
	source, err := r.GetSource("usda-air-fryer-safety")
	if err != nil || len(source.SHA256) != 64 {
		t.Fatal("source resolution failed")
	}
	if _, err := r.GetSource("absent"); err == nil {
		t.Fatal("unknown source accepted")
	}
}
func TestWorkflowInterruptResumeAndNoReplay(t *testing.T) {
	w := NewWorkflow(fixture(), localKnowledge{})
	r, err := w.Start(context.Background(), "historical success", "preference")
	if err != nil {
		t.Fatal(err)
	}
	id := r.RunID
	if r.Status != "interrupted" || r.Decision != Ask || r.Guidance != nil || len(r.Events) != 3 || r.Interrupt == nil {
		t.Fatal("default path released action")
	}
	// Caller mutation must not alter the checkpoint's offered hypotheses.
	r.Interrupt.Offered[0].Eligible = true
	r, err = w.Resume(context.Background(), id, HumanEvidence{Selected: "korean_spiced_soup_food", AcceptsUnknowns: Bool(true), Authorizes: Bool(true)})
	if err != nil || r.Status != "interrupted" || r.Guidance != nil {
		t.Fatal("unknown hypothesis advanced")
	}
	r, err = w.Resume(context.Background(), id, authorized())
	if err != nil {
		t.Fatal(err)
	}
	if r.RunID != id || r.Status != "completed" || r.Decision != AllowBounded || r.Guidance == nil || len(r.Events) != 7 {
		t.Fatalf("bad completion: %+v", r)
	}
	if r.Events[0].Stage != PerceiveStage || r.Events[5].Stage != ActStage || r.Events[6].Stage != LearnStage {
		t.Fatal("unexpected stage order")
	}
	if _, err = w.Resume(context.Background(), id, authorized()); err == nil {
		t.Fatal("completed run replayed")
	}
}
func TestWorkflowConcernStops(t *testing.T) {
	w := NewWorkflow(fixture(), localKnowledge{})
	r, err := w.Start(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	r, err = w.Resume(context.Background(), r.RunID, HumanEvidence{Concern: true})
	if err != nil || r.Status != "stopped" || r.Guidance != nil || len(r.Events) != 4 {
		t.Fatal("concern did not stop")
	}
}
func TestPublicTraceExcludesHiddenFields(t *testing.T) {
	w := NewWorkflow(fixture(), localKnowledge{})
	r, err := w.Start(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"system_prompt", "chain_of_thought", "hidden_reasoning", "/Users/"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("public trace contains %s", forbidden)
		}
	}
	if !strings.Contains(string(data), "model_assertion") || !strings.Contains(string(data), "mention_id") {
		t.Fatal("trace lost assertion sources or unresolved mentions")
	}
}
func TestRealStdioMCPWorkflow(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	w := NewWorkflow(fixture(), MCPKnowledgePort{Executable: exe})
	r, err := w.Start(ctx, "historical result", "preference")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(r.Events[2].Evidence)
	if err != nil {
		t.Fatal(err)
	}
	var reason ReasonResult
	if err = json.Unmarshal(data, &reason); err != nil {
		t.Fatal(err)
	}
	call := reason.Knowledge.MCPCall
	if !call.Success || call.ProtocolVersion == "" || call.ServerName != ServerName || call.ServerVersion != Version || call.ToolName != "search_knowledge" || len(reason.Knowledge.Chunks) != 3 {
		t.Fatalf("missing real MCP proof: %+v", call)
	}
	r, err = w.Resume(ctx, r.RunID, authorized())
	if err != nil || r.Status != "completed" || len(r.Events) != 6 {
		t.Fatalf("real MCP end to end failed: %v %+v", err, r)
	}
}
func TestMCPSourceResolutionAndToolErrors(t *testing.T) {
	r, err := NewRetriever()
	if err != nil {
		t.Fatal(err)
	}
	server := NewMCPServer(r)
	st, ct := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_source", Arguments: SourceArgs{"usda-air-fryer-safety"}})
	if err != nil || result.IsError || result.StructuredContent == nil {
		t.Fatal("MCP source resolution failed", err)
	}
	for _, args := range []SearchArgs{{"fish", 6}, {"", 3}} {
		result, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "search_knowledge", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Fatal("MCP accepted invalid retrieval arguments")
		}
	}
}

func TestDurableProofConcurrentReviewAndActiveClaim(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenProofStore(dir, "concurrent-001", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = RunProof(context.Background(), "proof-1", s, "", fixture(), localKnowledge{}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	a, err := OpenProofStore(dir, "concurrent-001", false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := OpenProofStore(dir, "concurrent-001", false)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, store := range []*ProofStore{a, b} {
		go func(store *ProofStore) {
			<-start
			_, err := RunProof(context.Background(), "proof-2", store, "APPROVE", fixture(), localKnowledge{})
			results <- err
		}(store)
	}
	close(start)
	success := 0
	for range 2 {
		if <-results == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent review successes = %d, want 1", success)
	}
	r, err := a.Read("adjudicated")
	if err != nil || len(r.Events) != 4 {
		t.Fatal("concurrent review replayed a stage", err)
	}
	if err = a.claim("adjudicated", "executing"); err != nil {
		t.Fatal(err)
	}
	if _, err = RunProof(context.Background(), "proof-3", b, "", fixture(), localKnowledge{}); err == nil {
		t.Fatal("active execution claim was replayed")
	}
}
