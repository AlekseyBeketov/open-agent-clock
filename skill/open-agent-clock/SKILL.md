---
name: open-agent-clock
description: Use when configuring or operating explicitly authorized local terminal-agent schedules with open-agent-clock on macOS.
---

# open-agent-clock

Use this skill when a user wants to configure, inspect, test, or operate explicitly authorized local invocations of installed terminal agent CLIs on macOS.

## Communication

- Communicate with the user in the language they normally use or explicitly prefer.
- Infer that language from the current conversation when it is clear; ask only when it is genuinely ambiguous.
- Preserve commands, identifiers, flags, paths, and technical terms exactly when translating the surrounding explanation.

## Tool responsibility

- On supported macOS systems, operate the installed `open-agent-clock` CLI. The skill teaches detection, preview, configuration, scheduling, updates, notifications, and diagnostics; it does not replace the CLI with the current agent's own cron or recurring-task facility.
- If the host, provider, or execution path is unsupported, state that boundary first. A host-native scheduler may be offered only as a clearly separate fallback and must not be presented as an `open-agent-clock` feature.
- Describe context reduction narrowly: the CLI controls its prompt, temporary working directory, ephemeral mode where supported, project rules, and optional toolsets. It cannot erase provider-side system prompts, provider-managed history, or guarantee billed-token savings.

## Safety contract

- Describe the tool as a transparent scheduler or usage-window planner, never as a limit resetter or bypasser.
- API-key targets must not participate in usage-window schedules.
- Credentials remain in the provider CLI's own auth store. Never copy, print, persist, or expose secret values.
- Use detection and dry-run first. Do not invoke Codex, Claude, Hermes, or another provider without separate explicit owner confirmation.
- State that a scheduled invocation may consume subscription allowance, fail, or be rejected.
- Never claim that a run guarantees a server-side reset time, additional capacity, or provider acceptance.
- Do not automatically retry usage-limit or rate-limit failures.
- The native Codex plan uses `codex --ask-for-approval never exec --ephemeral --sandbox read-only --skip-git-repo-check hi` because the provider runs in a temporary empty cwd. Only `run --once --confirm --dev` (or `--diagnostic`) adds `--json` before the prompt for JSONL usage; ordinary `run --once`, `tick`, and scheduled `launchd` invocations remain unchanged.

## Workflow

1. Run `open-agent-clock detect` and inspect each binding independently.
2. Treat native Codex and Hermes with `openai-codex` as separate execution bindings, but do not infer different server accounts from different local credential stores.
3. Create a disabled schedule with `schedule add` or the guided setup.
4. Review the schedule with `status` and `run --dry-run`.
5. Show the target, backend, auth mode, command, temporary working-directory policy, timezone, prompt, and next run before activation.
6. Enable or resume a provider schedule only after explicit confirmation.
7. Preview the LaunchAgent with `schedule install --dry-run`.
8. Install only the selected user-level LaunchAgent after explicit confirmation.
9. Use `history`, `last-run`, `status`, and `schedule status` for observability.
10. For provider-reported usage, use only a real confirmed one-off dev run and inspect it with `last-run --dev` or `last-run --diagnostic`.
11. Do not persist or display raw provider JSONL, prompt, response, stdout, or stderr; absent usage is `availability: unavailable`.
12. Pause or uninstall by managed schedule ID; never modify provider credentials.
13. After any change, report exactly what changed and whether a provider process was invoked.

## Claude boundary

Do not use API-key or `--bare` fallbacks for Claude usage-window scheduling. If a subscription-compatible local path cannot be confirmed, leave the capability unsupported. The provider's official native scheduling feature may be mentioned as a separate alternative, not as an `open-agent-clock` execution binding.

## Defaults

- Interval: `5h3m`
- Minimal prompt: `hi`
- Morning example: `05:00` in the selected IANA timezone
- Missed LaunchAgent runs: skip rather than catch up

## Useful commands

```bash
open-agent-clock detect
open-agent-clock status
open-agent-clock run --dry-run --target native-codex --prompt hi
open-agent-clock schedule status
open-agent-clock history
```

Before any real invocation or LaunchAgent installation, explain the side effect and obtain explicit confirmation.
