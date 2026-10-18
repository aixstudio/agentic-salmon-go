# Validation record

Initial port validated on 2026-10-17 on macOS ARM64.

## Source provenance

This repository was ported from the public Python repository
[`aixstudio/agentic-salmon-python`](https://github.com/aixstudio/agentic-salmon-python)
at commit [`8eb404dfa25d05628b02749f64e9dcd192d66495`](https://github.com/aixstudio/agentic-salmon-python/commit/8eb404dfa25d05628b02749f64e9dcd192d66495)
(source version 0.2.1). Only curated public assets and source contracts were
used; the new repository has independent Git history.

## Python baseline

- Both the engineering source and public checkout passed all 33 existing tests
  with Python 3.13 and strict LangGraph MessagePack serialization enabled.
- A fresh allowlisted public export matched the public checkout byte for byte.
- The public checkout matched its published GitHub HEAD.
- Both checkouts executed default interruption and authorized completion with
  real FastEmbed retrieval through the official Python MCP SDK over stdio.
- The public checkout used its own source tree and the engineering workspace's
  installed Python 3.13 dependencies; a fresh dependency installation was not
  part of this baseline check.

## Go port

- `go test ./...` passed under Go 1.25.0 and Go 1.27.2.
- `go test -race -coverprofile=coverage.out ./...` passed under Go 1.27.2.
  Coverage: CLI 83.0%, workflow package 89.1%.
- Seventeen top-level tests with table-driven cases cover all six agent
  contracts, fixture/image binding, unknown fields, ontology rejection,
  hypothesis status, missing/refused authority, retained unknowns, citation
  integrity, ranking, source resolution, pause/resume, non-replay, CLI paths,
  and actual SDK client/server tool calls.
- `go vet ./...`, `go mod verify`, formatting, and binary compilation passed.
- Real compiled CLI runs exercised default interruption and authorized
  six-stage completion using the embedded corpus and a separate MCP server
  process. JSON showed server version, negotiated protocol, success, scores,
  source URLs, and citation identifiers.
- Python/Go comparisons matched stage order, decisions, guidance, perception,
  connected ontology event, hypothesis statuses/evidence, and retained unknowns
  for the default and authorized demonstrations. Retrieval algorithms, query
  wording, scores, ranking, timings, and run identifiers intentionally differ.
- Public-content checks rejected private paths and credential patterns; the
  copied JPEG passed the existing EXIF/GPS metadata check. Local run reports,
  compiled binaries, and private development material are excluded from Git.

These checks establish the stated local demo behavior. CI performs its own
Linux formatting, dependency, vet, race-test, build, and real stdio MCP checks.
No production deployment, hosted approval service, or dense semantic retrieval
equivalence is claimed for the Go port.

## Durable HITL revision — 2026-10-18

The CLI is now three incremental proof commands with one run ID and local SQLite persistence. Separate-process tests establish: Proof-1 saves exactly three stages; APPROVE in Proof-2 saves Review as stage four and releases no guidance; Proof-3 executes Act/Learn and records completion with six stages. Tests compare saved evidence to prove no repeated perception/retrieval, reject invalid review input without consuming the pause, enforce STOP/EOF, block wrong-order commands and reject repeated Act. The proof flow has no optional/free-text evidence fields.

The corresponding Python source uses the official LangGraph async SQLite checkpointer, preserving its LangGraph and dense semantic retrieval implementation. Both runners produce bounded terminal guidance and detailed ignored log/<run-id>.log artifacts. Historical outcome evidence is fixed and labeled; no new cooking outcome is asserted.
