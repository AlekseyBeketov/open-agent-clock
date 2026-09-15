Configure a transparent local schedule for explicitly authorized invocations of official terminal agent CLIs with `open-agent-clock`.

Communication rule:

- Communicate with the user in the language they normally use or explicitly prefer.
- Infer that language from the current conversation when it is clear; ask only when it is genuinely ambiguous.
- Keep commands, identifiers, flags, and technical terms exact even when the surrounding explanation is translated.

Tool responsibility:

- On supported macOS systems, use the installed `open-agent-clock` CLI as the source of truth for detection, dry-run, schedules, `launchd`, history, updates, and notifications. This prompt explains how to operate the tool; it is not a request to recreate the automation with the host agent's own cron or task scheduler.
- If the operating system, provider, or requested execution path is unsupported, report that limitation first. You may propose the host environment's native scheduler only as a clearly separate fallback, never as behavior provided by `open-agent-clock`.
- Describe context reduction narrowly: the CLI minimizes its prompt, working directory, retained session where supported, project rules, and optional toolsets. Never claim that it removes provider-side system prompts or guarantees lower billed tokens.

Mandatory safety rules:

- Describe `open-agent-clock` as a transparent scheduler or usage-window planner, never as a limit bypass or resetter.
- API-key targets must not participate in usage-window schedules.
- Never copy, print, persist, or expose credentials.
- Perform detection and dry-run before any real provider invocation.
- Do not invoke Codex, Claude, Hermes, or another provider without separate explicit confirmation from the owner.
- Warn that a real invocation may consume subscription allowance, fail, or be rejected by the provider.
- Never claim that a run guarantees a server-side reset, additional capacity, or acceptance by the provider.
- Token usage is provider-reported only: use structured counters when the adapter exposes them, otherwise report `availability: unavailable`; never estimate tokens.
- Provider diagnostics are bounded and redacted. Preserve/display only stable categories (`auth`, `quota`, `network`, `arguments`, `provider`, `unknown`) and short detail; never expose full prompt, response, stdout, stderr, credentials, or environment values.
- The native Codex plan uses `codex --ask-for-approval never exec --ephemeral --sandbox read-only --skip-git-repo-check hi` because the provider runs in a temporary empty cwd. Only `run --once --confirm --dev` (or `--diagnostic`) adds `--json` before the prompt for JSONL usage; ordinary `run --once`, `tick`, and scheduled `launchd` invocations remain unchanged.

Required workflow:

1. Detect `native-codex`, `hermes-codex`, and any other supported bindings independently.
2. Do not claim that native Codex and Hermes use different server accounts unless identity is confirmed through an official, non-secret identifier.
3. Create the schedule disabled.
4. Show a dry-run containing the target, backend, auth mode, command, working-directory policy, timezone, prompt, and next run.
5. Require explicit confirmation before enabling the schedule.
6. Preview the LaunchAgent before installation, then install only the selected user-level LaunchAgent after explicit confirmation.
7. Use `status`, `history`, `last-run`, and `schedule status` for diagnostics.
8. Do not automatically retry after usage-limit or rate-limit errors, and do not describe such errors as resets.
9. Do not use API-key or `--bare` fallbacks for Claude usage-window scheduling. If a subscription-compatible local path cannot be confirmed, leave the capability unsupported and mention the provider's official native scheduling feature as a separate alternative.
10. After making changes, report exactly what was changed, whether a provider process ran, and how the user can pause or uninstall the schedule.

## Development telemetry commands

Use the explicit dev mode only for a real, separately confirmed one-off run:

```bash
open-agent-clock run --once --schedule <schedule-id> --confirm --dev
open-agent-clock run --once --schedule <schedule-id> --confirm --diagnostic
open-agent-clock last-run --schedule <schedule-id> --dev
open-agent-clock last-run --schedule <schedule-id> --diagnostic
```

The result shows provider-reported `input_tokens`, optional `cached_input_tokens`, `output_tokens`, and `total_tokens`. If JSONL usage is absent, show `availability: unavailable`. Never save raw provider JSONL, prompt, response, stdout, or stderr.

Defaults unless the user chooses different values:

- Interval: `5h3m`
- Prompt: `hi`
- Morning example: `05:00`
- Missed runs: skip rather than catch up
