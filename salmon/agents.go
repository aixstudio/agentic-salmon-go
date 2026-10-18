package salmon

import (
	"context"
	"fmt"
	"strings"
)

// KnowledgePort is the replaceable capability owned by Reason.
type KnowledgePort interface {
	Search(context.Context, string, int) (KnowledgeResult, error)
}
type ReasonAgent struct{ Knowledge KnowledgePort }

func (a ReasonAgent) Run(ctx context.Context, p Perception, c ConnectedEvent) (ReasonResult, error) {
	var out ReasonResult
	if a.Knowledge == nil {
		return out, fmt.Errorf("Reason requires a knowledge capability")
	}
	labels := []string{}
	for _, e := range c.Entities {
		if e.Kind == FoodPortion || e.Kind == CookingEquipment {
			labels = append(labels, e.Label)
		}
	}
	candidates := []string{}
	for _, h := range p.Candidates {
		candidates = append(candidates, h.Label)
	}
	query := "guidance for unidentified seasoned food portions with a visible skin-like surface in an air fryer; scene entities: " + strings.Join(labels, ", ") + "; retained unknowns: " + strings.Join(p.Unknowns, ", ") + "; source-bound candidate hypotheses: " + strings.Join(candidates, ", ") + "; food thermometer, bounded guidance, image evidence limits"
	k, err := a.Knowledge.Search(ctx, query, 3)
	if err != nil {
		return out, err
	}
	if len(k.Chunks) == 0 {
		return out, fmt.Errorf("Reason requested RAG but retrieved no chunks")
	}
	limiting := []string{}
	for _, chunk := range k.Chunks {
		t := strings.ToLower(chunk.Text)
		if strings.Contains(t, "cannot verify species") || strings.Contains(t, "image without original packaging") {
			limiting = append(limiting, chunk.ID)
		}
	}
	observations := map[string]Observation{}
	for _, o := range p.Observations {
		observations[o.ID] = o
	}
	out = ReasonResult{Query: query, Unknowns: append([]string{}, p.Unknowns...), Knowledge: k, Hypotheses: []Hypothesis{}, ProposedAction: "Ask the human to select a working hypothesis, accept retained unknowns, and explicitly authorize only bounded guidance grounded in retrieved sources."}
	for _, candidate := range p.Candidates {
		h := Hypothesis{ID: candidate.ID, Name: candidate.Label, Status: Unknown, Claims: []string{candidate.Claim}, EvidenceIDs: []string{"candidate:" + candidate.ID}, SupportingChunkIDs: []string{}, LimitingChunkIDs: []string{}}
		if candidate.Source != HumanHypothesis && candidate.Source != ModelAssertion {
			return out, fmt.Errorf("invalid candidate source")
		}
		for _, id := range candidate.SupportingIDs {
			if o, ok := observations[id]; ok {
				h.Claims = append(h.Claims, o.Claim)
				h.EvidenceIDs = append(h.EvidenceIDs, id)
			} else {
				return out, fmt.Errorf("candidate references absent observation")
			}
		}
		if len(h.EvidenceIDs) > 1 {
			if candidate.Source == ModelAssertion {
				h.Status = Plausible
			} else {
				h.Status = Supported
			}
		}
		if candidate.Source == ModelAssertion {
			h.LimitingChunkIDs = append(h.LimitingChunkIDs, limiting...)
		}
		out.Hypotheses = append(out.Hypotheses, h)
	}
	if len(out.Hypotheses) == 0 {
		return out, fmt.Errorf("Reason requires source-bound hypotheses")
	}
	return out, nil
}

type ReviewAgent struct{}

func (ReviewAgent) Run(r ReasonResult, e HumanEvidence) ReviewResult {
	out := ReviewResult{Decision: Ask, Selected: e.Selected, Unknowns: []string{}, Resolved: []string{}, Reasons: []string{}}
	for _, u := range r.Unknowns {
		if u == "odor after opening" && strings.TrimSpace(e.Odor) != "" {
			out.Resolved = append(out.Resolved, u)
		} else {
			out.Unknowns = append(out.Unknowns, u)
		}
	}
	if e.Concern {
		out.Decision = Stop
		out.Reasons = []string{"The human reported a concern; bounded guidance is blocked."}
		return out
	}
	if e.Authorizes != nil && !*e.Authorizes {
		out.Decision = Stop
		out.Reasons = []string{"The human declined bounded guidance."}
		return out
	}
	if e.Selected != "" {
		var selected *Hypothesis
		for i := range r.Hypotheses {
			if r.Hypotheses[i].ID == e.Selected {
				selected = &r.Hypotheses[i]
			}
		}
		if selected == nil {
			out.Reasons = []string{"Select a hypothesis offered by Reason."}
			return out
		}
		if selected.Status != Supported && selected.Status != Plausible {
			out.Reasons = []string{"An unknown hypothesis cannot authorize action; select a supported or plausible hypothesis, or stop."}
			return out
		}
	} else {
		out.Reasons = append(out.Reasons, "human-selected working hypothesis required")
	}
	if e.AcceptsUnknowns == nil || !*e.AcceptsUnknowns {
		out.Reasons = append(out.Reasons, "explicit acceptance of retained uncertainty required")
	}
	if e.Authorizes == nil || !*e.Authorizes {
		out.Reasons = append(out.Reasons, "explicit authorization for bounded guidance required")
	}
	if len(out.Reasons) == 0 {
		out.Decision = AllowBounded
		out.Reasons = []string{"Human selected a working hypothesis, retained unknowns, and authorized bounded cited guidance."}
	}
	return out
}

type ActAgent struct{}

func (ActAgent) Run(r ReasonResult, v ReviewResult) (ActionResult, error) {
	var out ActionResult
	if v.Decision != AllowBounded {
		return out, fmt.Errorf("Act requires ALLOW_BOUNDED")
	}
	var selected *Hypothesis
	for i := range r.Hypotheses {
		h := &r.Hypotheses[i]
		if h.ID == v.Selected && (h.Status == Supported || h.Status == Plausible) {
			selected = h
		}
	}
	if selected == nil {
		return out, fmt.Errorf("Act requires the reviewed supported or plausible hypothesis")
	}
	out.CitationIDs = []string{}
	for _, c := range r.Knowledge.Chunks {
		if strings.Contains(c.Text, "145") && strings.Contains(strings.ToLower(c.Text), "thermometer") && c.ID != "" && c.URL != "" && c.SHA256 == digest([]byte(c.Text)) {
			out.CitationIDs = append(out.CitationIDs, c.ID)
		}
	}
	if len(out.CitationIDs) == 0 {
		return out, fmt.Errorf("Act requires a retrieved intact citation for its 145°F guidance")
	}
	out.Guidance = "Identity remains unverified. If you proceed under your human-selected " + selected.Name + " hypothesis, do not rely on appliance time or appearance alone; measure the center with a food thermometer and use the cited 145°F fish guidance. Stop if you have a storage, odor, allergy, or identity concern."
	out.Authority = "human-authorized bounded guidance"
	return out, nil
}

type LearnInput struct {
	Action            ActionResult
	Outcome, Feedback string
}
type LearnAgent struct{}

func (LearnAgent) Run(in LearnInput) (LearnResult, error) {
	if len(in.Action.CitationIDs) == 0 || in.Action.Authority != "human-authorized bounded guidance" {
		return LearnResult{}, fmt.Errorf("Learn requires the preceding cited authorized action")
	}
	return LearnResult{in.Outcome, in.Feedback, "none; outcome evidence cannot weaken safety or authority policy"}, nil
}
