# ADR 0001 — Controlled workflow in native Go

Status: accepted for this demonstration.

## Context

The Python sibling separates evidence, connection, reasoning, human authority,
action, and learning. A language port must preserve those boundaries and prove
real retrieval over MCP without claiming Python-only infrastructure runs in Go.

## Decision

Implement six typed agent contracts, a separate Guard, and a native SQLite-backed resumable
state machine. Review pauses after Reason. Each resume validates offered
hypotheses, acceptance of uncertainty, and explicit bounded authority. Failed
validation pauses again; concern or refusal stops. Act validates selection and
retrieved citation content. Learn cannot change policy.

Use the official Model Context Protocol Go SDK with stdio subprocess transport.
Reason owns the knowledge capability; the workflow configures its executable.
Expose read-only `search_knowledge` and `get_source` tools.

Index the six reviewed chunks as TF-IDF sparse vectors and rank by cosine
similarity, with stable chunk-ID tie breaking. Return only positive matches and
retain source URLs, SHA-256 digests, and scores. Reason consumes these results;
Act fails when its required citation is absent. This is lexical RAG. It does
not reproduce Python's FastEmbed dense semantic retrieval or numerical scores.

## Consequences

The runnable demo needs only Go and downloaded Go modules. It has real protocol
and retrieval boundaries, deterministic agent contracts, and auditable results.
Checkpoints survive process exits. The three proofs persist a review pause, an
adjudicated pre-action pause, and completion. They do not provide production
authorization identity. Python LangGraph checkpoint serialization
and dense embeddings are deliberately not presented as ported features.

Content digests identify the reviewed text used by a run and detect mismatches
between returned text and digest. They do not authenticate a malicious server.
Public source-policy citations resolve to this ADR: images cannot verify food
identity or safety; only human-authorized bounded guidance may reach Act.

## Incremental HITL proof flow

Proof-1 saves Perceive/Connect/Reason and the review pause. Proof-2 reopens the same run, accepts only APPROVE or STOP/Enter, records Review, and saves a pre-action adjudication without running Act/Learn. Proof-3 consumes that adjudication and executes Act/Learn once, using fixed historical case outcome/preference evidence. No guessed storage/odor text is requested and no unknown is erased.

Pure-Go SQLite and conditional phase updates protect the pickup boundary from wrong-order, concurrent and repeated execution. A crashed active claim is blocked from automatic replay. Full run artifacts are appended to log/<run-id>.log; console output stays focused on review guidance and watch-outs.
