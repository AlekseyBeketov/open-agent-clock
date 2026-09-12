Configure a transparent local schedule for explicitly authorized invocations of official terminal agent CLIs with `open-agent-clock`.

Communication rule:

- Communicate with the user in the language they normally use or explicitly prefer.
- Infer that language from the current conversation when it is clear; ask only when it is genuinely ambiguous.
- Keep commands, identifiers, flags, and technical terms exact even when the surrounding explanation is translated.

Mandatory safety rules:

- Describe `open-agent-clock` as a transparent scheduler or usage-window planner, never as a limit bypass or resetter.
- API-key targets must not participate in usage-window schedules.
- Never copy, print, persist, or expose credentials.
- Perform detection and dry-run before any real provider invocation.
- Do not invoke Codex, Claude, Hermes, or another provider without separate explicit confirmation from the owner.
- Warn that a real invocation may consume subscription allowance, fail, or be rejected by the provider.
- Never claim that a run guarantees a server-side reset, additional capacity, or acceptance by the provider.

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

Defaults unless the user chooses different values:

- Interval: `5h3m`
- Prompt: `hi`
- Morning example: `05:00`
- Missed runs: skip rather than catch up
