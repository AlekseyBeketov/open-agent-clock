# Repository Agent Instructions

## Engineering Boundaries

- Treat external API, file, config, and tool payloads as untrusted. Validate unknown values once at the boundary that owns them, then pass typed values inward; do not repeat checks guaranteed by a schema, constructor, or internal type. A cast or type assertion alone is not runtime validation. Keep authorization, security checks, and mutable business invariants enforced where required.
- Before adding speculative reliability or edge-case behavior, state the concrete failure mode, evidence or uncertainty about its likelihood, and complexity/maintenance cost, then get the user's approval. Do not invent probabilities. Track out-of-scope proposals as explicit pending decisions; no reply is not consent. Mandatory security controls are not optional.

## Go Conventions

- Model internal state with concrete structs and typed interfaces; keep `any`/`interface{}` at explicitly justified I/O boundaries, validate decoded values there, and convert them to domain types.
- Handle returned errors explicitly; prefer early returns and wrap errors with `%w` when adding operation context. Preserve stable domain errors where callers classify them; document any intentionally ignored best-effort failure.
- Use Go error returns for expected failures; do not replace them with panic/recover flows.
- Add interfaces or helpers only for meaningful package/adapter boundaries or demonstrated reuse; avoid speculative abstractions and one-use wrappers.
- Test observable production behavior through the existing package tests; do not duplicate logic in tests or make live provider calls that may consume subscription allowance.

## Verification

- Use existing `make test`, `make vet`, and `make build` targets; the CI formatting check is `test -z "$(gofmt -l cmd internal)"`.
