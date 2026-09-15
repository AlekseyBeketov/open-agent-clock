# Roadmap

This file records possible future work. Nothing here is scheduled for implementation until the owner explicitly requests it.

## Native macOS menu-bar application — deferred

- Build a native application that lives in the macOS menu bar/control area.
- Provide schedule, target, update, notification, status, and recent-run controls without hiding the underlying CLI behavior.
- Design the interface with Apple-like clarity and restraint, combined with OpenAI-style typography and interaction patterns.
- Preserve the current safety model: visible execution plans, explicit consent for provider calls and updates, no credential copying, and managed LaunchAgents only.
- Define the implementation stack, visual system, accessibility requirements, and migration path in a separate OpenSpec change when requested.

## Dev token usage and provider response telemetry — deferred

- Add an explicit development/diagnostic mode that displays token usage for each provider response to the minimal `hi` prompt.
- Show input tokens, output tokens, total tokens, duration, exit status, and the source of the metric when the provider exposes them.
- Preserve a clear `unavailable` state when a CLI does not return usage metadata; never invent or estimate token counts silently.
- Store only non-secret telemetry in local history and expose it through `history`, `last-run`, and a machine-readable JSON view.
- Keep the feature opt-in and separate from normal provider execution; do not include prompt contents, credentials, or full provider output in telemetry.
- Investigate provider-specific sources first: Codex CLI structured output/JSON or local usage metadata, then Hermes/OpenAI-compatible response usage fields. Define separate adapters rather than assuming all CLIs expose the same counters.
- **Token Reduction Strategy**: Implement a `minimal-context` mode for scheduled runs to reduce input tokens. This should include:
    - Disabling user-level MCP servers (`-c mcp_servers.<name>.enabled=false`).
    - Ignoring user and project rules (`--ignore-user-config`, `--ignore-rules`).
    - Setting `model_reasoning_effort` to `minimal` and `model_verbosity` to `low`.
    - Continuing to use `--ephemeral` and `--skip-git-repo-check`.

