# open-agent-clock skill

Use this skill when the user wants to plan explicitly authorized local invocations of installed terminal agent CLIs on macOS.

## Safety contract

- Describe the tool as a transparent scheduler or usage-window planner, never as a limit resetter or bypasser.
- API-key targets не участвуют в usage-window schedules.
- Credentials remain under the provider CLI; do not copy, print, persist, or diagnose secret values.
- Use dry-run first. Реальные вызовы Codex/Claude до этого запускаться не будут without a separate explicit owner confirmation.
- State that a scheduled invocation may consume allowance, fail, or be rejected and does not guarantee a server-side reset time or additional capacity.

## Workflow

1. Run `open-agent-clock detect` and inspect bindings independently.
2. Treat native Codex and Hermes `openai-codex` as separate execution bindings; do not infer different accounts from different local stores.
3. Create a disabled schedule with `schedule add`.
4. Review it with `status` and `run --dry-run`.
5. Enable/resume only with explicit `--confirm`.
6. Preview LaunchAgent with `schedule install --dry-run`.
7. Install only the selected user-level job with `schedule install --confirm`.
8. Use `history`, `last-run`, and `schedule status` for observability.
9. Pause or uninstall by managed schedule ID; never touch provider credentials.

## Defaults

- Codex interval: `5h3m`.
- Minimal prompt: `hi`.
- Morning example: `05:00` in the system/IANA timezone.
- Missed launchd runs: skip by default.
