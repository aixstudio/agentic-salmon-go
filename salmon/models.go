// Package salmon implements the guarded six-agent workflow independently of its CLI.
package salmon

type Stage string

const (
	PerceiveStage Stage = "perceive"
	ConnectStage  Stage = "connect"
	ReasonStage   Stage = "reason"
	ReviewStage   Stage = "review"
	ActStage      Stage = "act"
	LearnStage    Stage = "learn"
)

type Decision string

const (
	Ask          Decision = "ask"
	AllowBounded Decision = "allow_bounded"
	Stop         Decision = "stop"
)

type HypothesisStatus string

const (
	Supported HypothesisStatus = "supported"
	Plausible HypothesisStatus = "plausible"
	Unknown   HypothesisStatus = "unknown"
)

type AssertionSource string

const (
	HumanHypothesis AssertionSource = "human_hypothesis"
	ModelAssertion  AssertionSource = "model_assertion"
)

type EntityKind string

const (
	FoodPortion       EntityKind = "food_portion"
	CookingEquipment  EntityKind = "cooking_equipment"
	ObservableFeature EntityKind = "observable_feature"
)

type RelationKind string

const (
	LocatedIn         RelationKind = "located_in"
	HasVisibleFeature RelationKind = "has_visible_feature"
)

type Observation struct {
	ID     string `json:"observation_id"`
	Claim  string `json:"claim"`
	Source string `json:"source"`
}
type CandidateHypothesis struct {
	ID            string          `json:"hypothesis_id"`
	Label         string          `json:"label"`
	Source        AssertionSource `json:"source_kind"`
	Claim         string          `json:"claim"`
	SupportingIDs []string        `json:"supporting_observation_ids"`
}
type EntityMention struct {
	ID    string     `json:"mention_id"`
	Kind  EntityKind `json:"entity_kind"`
	Label string     `json:"label"`
	Claim string     `json:"evidence_claim"`
}
type RelationshipMention struct {
	Subject string       `json:"subject_mention_id"`
	Kind    RelationKind `json:"relation_kind"`
	Object  string       `json:"object_mention_id"`
	Claim   string       `json:"evidence_claim"`
}
type Perception struct {
	Observations  []Observation         `json:"observations"`
	Unknowns      []string              `json:"unknowns"`
	Provenance    string                `json:"provenance"`
	ImageSHA256   string                `json:"input_image_sha256"`
	Candidates    []CandidateHypothesis `json:"candidate_hypotheses"`
	Entities      []EntityMention       `json:"entity_mentions"`
	Relationships []RelationshipMention `json:"relationship_mentions"`
}
type Entity struct {
	ID     string     `json:"entity_id"`
	Kind   EntityKind `json:"entity_kind"`
	Label  string     `json:"label"`
	Claims []string   `json:"evidence_claims"`
}
type Relationship struct {
	Subject string       `json:"subject_id"`
	Kind    RelationKind `json:"relation_kind"`
	Object  string       `json:"object_id"`
	Claim   string       `json:"evidence_claim"`
}
type Ontology struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type ConnectedEvent struct {
	ID            string         `json:"event_id"`
	Type          string         `json:"event_type"`
	Ontology      Ontology       `json:"ontology"`
	Entities      []Entity       `json:"entities"`
	Relationships []Relationship `json:"relationships"`
}
type Chunk struct {
	ID        string  `json:"chunk_id"`
	SourceID  string  `json:"source_id"`
	Title     string  `json:"title"`
	Publisher string  `json:"publisher"`
	URL       string  `json:"url"`
	Text      string  `json:"text"`
	Score     float64 `json:"score"`
	SHA256    string  `json:"content_sha256"`
}
type MCPCall struct {
	ServerName      string  `json:"server_name"`
	ServerVersion   string  `json:"server_version"`
	ProtocolVersion string  `json:"protocol_version"`
	ToolName        string  `json:"tool_name"`
	DurationMS      float64 `json:"duration_ms"`
	Success         bool    `json:"success"`
}
type KnowledgeResult struct {
	Query   string  `json:"query"`
	Model   string  `json:"model"`
	Chunks  []Chunk `json:"chunks"`
	MCPCall MCPCall `json:"mcp_call"`
}
type Hypothesis struct {
	ID                 string           `json:"hypothesis_id"`
	Name               string           `json:"name"`
	Status             HypothesisStatus `json:"status"`
	Claims             []string         `json:"evidence_claims"`
	EvidenceIDs        []string         `json:"evidence_ids"`
	SupportingChunkIDs []string         `json:"supporting_chunk_ids"`
	LimitingChunkIDs   []string         `json:"limiting_chunk_ids"`
}
type ReasonResult struct {
	Query          string          `json:"query"`
	Hypotheses     []Hypothesis    `json:"hypotheses"`
	Unknowns       []string        `json:"retained_unknowns"`
	ProposedAction string          `json:"proposed_action"`
	Knowledge      KnowledgeResult `json:"knowledge"`
}

// Pointer booleans distinguish absent human evidence from an explicit refusal.
type HumanEvidence struct {
	Selected        string `json:"selected_hypothesis"`
	AcceptsUnknowns *bool  `json:"accepts_retained_unknowns"`
	Authorizes      *bool  `json:"authorizes_bounded_guidance"`
	Concern         bool   `json:"concern_reported"`
	Storage         string `json:"storage_recollection"`
	Odor            string `json:"odor_observation"`
}
type ReviewResult struct {
	Decision Decision `json:"decision"`
	Reasons  []string `json:"reasons"`
	Unknowns []string `json:"retained_unknowns"`
	Selected string   `json:"selected_hypothesis"`
	Resolved []string `json:"resolved_unknowns"`
}
type ActionResult struct {
	Guidance    string   `json:"guidance"`
	Authority   string   `json:"authority"`
	CitationIDs []string `json:"citation_chunk_ids"`
}
type LearnResult struct {
	Outcome      string `json:"outcome"`
	Feedback     string `json:"preference_feedback"`
	PolicyEffect string `json:"policy_effect"`
}
type Event struct {
	Sequence int    `json:"sequence"`
	Stage    Stage  `json:"stage"`
	Summary  string `json:"summary"`
	Evidence any    `json:"evidence"`
}
type Metric struct {
	Stage      Stage   `json:"stage"`
	StartedAt  string  `json:"started_at"`
	EndedAt    string  `json:"ended_at"`
	DurationMS float64 `json:"duration_ms"`
}
type OfferedHypothesis struct {
	ID       string           `json:"hypothesis_id"`
	Label    string           `json:"label"`
	Status   HypothesisStatus `json:"status"`
	Eligible bool             `json:"action_eligible"`
}
type Interrupt struct {
	Question         string              `json:"question"`
	Messages         []string            `json:"validation_messages"`
	RequiredFields   []string            `json:"required_fields"`
	Unknowns         []string            `json:"retained_unknowns"`
	Offered          []OfferedHypothesis `json:"offered_hypotheses"`
	AllowedDecisions []string            `json:"allowed_decisions"`
}
type Telemetry struct {
	TotalMS    float64 `json:"total_duration_ms"`
	TokenUsage *int    `json:"token_usage"`
	Note       string  `json:"token_usage_note"`
}
type RunResult struct {
	RunID     string     `json:"run_id"`
	Status    string     `json:"status"`
	Decision  Decision   `json:"decision"`
	Guidance  *string    `json:"guidance"`
	Interrupt *Interrupt `json:"interrupt"`
	Telemetry Telemetry  `json:"telemetry"`
	Events    []Event    `json:"events"`
	Metrics   []Metric   `json:"metrics"`
}

func Bool(v bool) *bool { return &v }
