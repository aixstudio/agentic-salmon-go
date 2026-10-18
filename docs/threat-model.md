# Threat model

| Risk | Control and practical limit |
| --- | --- |
| Image or fixture mismatch | SHA-256 binds the reviewed fixture to the sanitized image; strict JSON rejects unknown fixture fields. A trusted reviewer is still needed to approve fixture content. |
| Upstream semantic claims | Source kinds and observation references are validated; Connect owns canonical IDs and permits only ontology-valid relationships. |
| Retrieval mistaken for permission | Knowledge is read-only. Guard and Review separately control progression and release authority. |
| Invalid or missing approval | Optional booleans distinguish absent evidence from refusal; unknown/unoffered hypotheses cannot advance. EOF fails closed. |
| Uncited action | Act requires the retrieved temperature citation and matching text digest. The configured local server remains a trust boundary. |
| Repeated action | A run can resume only while interrupted; completed, stopped, or failed runs cannot replay Act. |
| Tool or command injection | The runtime supplies the executable and fixed subcommand. Evidence never becomes a shell command or tool name. Retrieval does not execute source text. |
| Policy weakened by feedback | Learn records outcome and preferences and reports no policy effect; no policy mutation interface exists. |
| Accidental disclosure | Publish curated source and reviewed assets; traces, environment files, SQLite checkpoints, `log/`, binaries, and editor workspace files are ignored. Traces intentionally include human inputs and should be reviewed before sharing. |

This demo does not provide authenticated human identity, production approval identity, remote-server authentication, corpus signing, or production audit
retention. The CLI is a local demonstration of explicit authority contracts.

The three CLI proofs use durable local SQLite checkpoints and conditional phase claims. They can resume after a normal process exit. A crashed active claim is not automatically retried; this is a local demo, not a distributed exactly-once effects system. Approval input is the fixed APPROVE/STOP contract, with all uncertainty retained.
