# open-agent-clock

[Русский](README.ru.md)

[![CI](https://github.com/AlekseyBeketov/open-agent-clock/actions/workflows/ci.yml/badge.svg)](https://github.com/AlekseyBeketov/open-agent-clock/actions/workflows/ci.yml)

**A transparent macOS scheduler for official agent CLIs.**

`open-agent-clock` plans explicitly authorized local invocations of installed terminal agents such as Codex CLI and Hermes. It helps keep recurring work aligned with your own schedule while keeping the execution path visible, inspectable, and under your control.

> This is **not** a limit bypass or resetter. It does not promise a server-side reset, extra capacity, or that a provider will accept a request.

## Why open-agent-clock?

- Schedule separate local targets independently.
- Preview the exact command before anything runs.
- Use macOS `launchd` without a daemon or root privileges.
- Keep credentials in the provider's own auth store.
- Record redacted execution metadata for troubleshooting.
- Fail closed: real execution and schedule activation require explicit confirmation.

## Supported targets

| Target | Provider | Local execution path | Usage-window scheduling |
|---|---|---|---|
| `native-codex` | OpenAI Codex | Native `codex` CLI and `~/.codex` | Subscription only |
| `hermes-codex` | OpenAI Codex | Hermes CLI with `openai-codex` | Subscription only |
| `claude-subscription` | Claude | Detected only when a subscription/OAuth path is safely confirmed | Subscription only |

Native Codex and Hermes are intentionally separate targets. Two local credential stores do not prove that two different server accounts are being used; identity may remain `unknown`.

**API-key targets do not participate in usage-window schedules.**

## Agent prompt and skill

The CLI works on its own. The repository also includes two optional files for users who want an AI coding agent to configure or operate it safely:

Both artifacts are written in English for portability, but explicitly instruct the agent to communicate in the user's usual or preferred language.

> **Open the canonical prompt:** [`prompt/open-agent-clock.md`](prompt/open-agent-clock.md). Its full text intentionally lives in that separate file so it does not occupy the README screen.

- [`skill/open-agent-clock/SKILL.md`](skill/open-agent-clock/SKILL.md) — a reusable agent skill for detection, dry-run, schedule activation, LaunchAgent management, and diagnostics.

Preview the prompt before using it:

```bash
curl -fsSL https://raw.githubusercontent.com/AlekseyBeketov/open-agent-clock/main/prompt/open-agent-clock.md
```

Install the skill into a standard agent skills directory:

```bash
mkdir -p ~/.agents/skills/open-agent-clock
curl -fsSL https://raw.githubusercontent.com/AlekseyBeketov/open-agent-clock/main/skill/open-agent-clock/SKILL.md \
  -o ~/.agents/skills/open-agent-clock/SKILL.md
```

Use your agent's own skills directory when it differs—for example, `~/.codex/skills/open-agent-clock/` for Codex CLI or `~/.hermes/skills/open-agent-clock/` for Hermes. Read the file before installing it, just as you would review `install.sh`.

### Context and token scope

`open-agent-clock` minimizes the **local invocation context**: the default prompt is only `hi`, provider processes start in a temporary empty working directory, native Codex uses `--ephemeral` when the installed CLI supports it, and Hermes runs one-shot with project rules ignored and optional toolsets empty. This avoids attaching this repository, an earlier interactive conversation, or unrelated agent skills through the invocation constructed by this tool.

It does **not** reset or erase a provider CLI's built-in system prompt, provider-managed metadata, or any history outside the behavior guaranteed by that CLI's documented flags. It also cannot reset usage windows or guarantee a particular context-window or billed-token reduction. Inspect `run --dry-run` to see exactly what this tool controls.

## Install

### Recommended: one command

On macOS, the installer selects the correct architecture, verifies the release checksum, installs the binary into `~/.local/bin`, and opens guided setup:

```bash
curl -fsSL https://raw.githubusercontent.com/AlekseyBeketov/open-agent-clock/main/install.sh | sh
```

Review [`install.sh`](install.sh) before piping it to a shell if that matches your security policy. To install without opening setup:

```bash
curl -fsSL https://raw.githubusercontent.com/AlekseyBeketov/open-agent-clock/main/install.sh | sh -s -- --no-setup
```

### Install with Go

Requires macOS and Go 1.27+:

```bash
go install github.com/AlekseyBeketov/open-agent-clock/cmd/open-agent-clock@latest
```

Then open guided setup:

```bash
open-agent-clock setup
```

If your shell cannot find the command, add Go's bin directory to `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

### Build from a clone

```bash
git clone https://github.com/AlekseyBeketov/open-agent-clock.git && cd open-agent-clock && make build
```

The binary will be available at `bin/open-agent-clock`.

### Try from a checkout without installing

```bash
go run ./cmd/open-agent-clock detect
```

Published releases include checksum-verified macOS arm64 and amd64 archives. If a requested release asset does not exist, the installer can fall back to a source build when Go 1.27+ is available. Package-manager distribution is planned for later.

## Updates

Check release metadata without changing the executable, inspect automatic-update state, or install explicitly:

```bash
open-agent-clock update check
open-agent-clock update status
open-agent-clock update install --confirm
open-agent-clock update disable
```

Interactive setup and **Settings → Automatic updates** offer an opt-in after an interval schedule exists. Enabling it attaches one update attempt to that schedule's existing cadence and phase; it does not create a second timer. Immediately before a confirmed due tick, the tool makes at most one bounded attempt, records a non-secret result, and then continues the provider tick even if the update check fails. Disabled is the default, including migrated configurations.

Installation is allowed only for released semantic versions and directly owned writable macOS binaries. The updater selects the exact `darwin` architecture asset, requires `checksums.txt`, verifies SHA-256 before extraction, rejects links and unsafe archive paths, stages the executable in the same directory, and replaces it atomically. Development builds, package-manager paths, missing checksums, invalid archives, network failures, and unwritable directories fail closed without replacing the working binary. It never uses `sudo` and never loops retries inside one occurrence.

## macOS completion notifications

Guided setup and **Settings → Completion notifications** offer an explicit opt-in; notifications are disabled by default, including for migrated configurations. Disable them later through the same Settings item.

After a real provider attempt completes, the tool makes at most one bounded best-effort request through macOS's built-in `/usr/bin/osascript`. The notification contains only the managed schedule ID, target ID, and `success`/`failure` classification. It never contains the prompt, credentials, provider stdout/stderr, or diagnostic reason. macOS may ask for notification permission or suppress delivery according to System Settings and Focus. Delivery failure is reported to stderr but does not change the persisted provider result and never retries the provider.

## Quick start: guided terminal setup

Start the application with either command:

```bash
open-agent-clock
# or explicitly:
open-agent-clock setup
```

The first screen asks for language, with English preselected. The wizard then discovers locally authenticated subscription targets and guides you through schedule mode, timing, timezone, prompt, preview, activation, and optional LaunchAgent installation. It never asks for credentials and creates the schedule disabled before any activation confirmation.

After setup, run `open-agent-clock` or `open-agent-clock ui` to manage targets, schedules, previews, executions, LaunchAgents, status, history, and settings through the menu.

### Accessible line-oriented mode

For screen readers or terminals that do not handle full-screen forms reliably:

```bash
ACCESSIBLE=1 open-agent-clock setup
ACCESSIBLE=1 open-agent-clock ui
```

### Direct commands for automation

Every operation remains available as a non-interactive command. For example:

```bash
open-agent-clock init
open-agent-clock detect
open-agent-clock schedule add --id codex-window --target native-codex --mode interval --interval 5h3m --timezone Europe/Moscow
open-agent-clock run --dry-run --target native-codex --prompt hi
open-agent-clock resume --id codex-window --confirm
open-agent-clock schedule install --id codex-window --confirm
```

`schedule add` creates a disabled schedule by default. `--confirm` is required when enabling a provider schedule or loading its LaunchAgent. Running without arguments in a non-interactive context prints help and never waits for TUI input.

## Schedule modes

### Interval

Run at a recurring interval. The default interval is `5h3m`:

```bash
open-agent-clock schedule add \
  --id codex-window \
  --target native-codex \
  --mode interval \
  --interval 5h3m \
  --timezone Europe/Moscow
```

### Morning

Run once per day at a chosen local time:

```bash
open-agent-clock schedule add \
  --id hermes-morning \
  --target hermes-codex \
  --mode morning \
  --time 05:00 \
  --timezone Europe/Moscow
```

### Custom daily times

```bash
open-agent-clock schedule add \
  --id daily-check \
  --target native-codex \
  --mode daily \
  --times 05:00,13:00,21:00 \
  --timezone Europe/Moscow
```

Schedules are timezone-aware and use IANA timezone names. Missed `launchd` runs are skipped; they are not automatically caught up.

### Launchd trigger materialization and limitations

- Interval schedules are materialized as a one-minute `StartInterval` polling trigger. The CLI's `tick` command applies the configured interval from the schedule start or last completed run, so loading the LaunchAgent does not shift the interval anchor.
- Morning/custom daily schedules use native `StartCalendarInterval` entries when their IANA timezone matches the macOS system timezone.
- `StartCalendarInterval` has no timezone field. For a daily schedule in another IANA timezone, the CLI uses the same one-minute polling trigger and evaluates the configured timezone itself instead of silently scheduling at the wrong local time.
- An interval shorter than one minute cannot be materialized for launchd and is rejected by `schedule install`. A single plist cannot combine interval and calendar triggers.

### Claude's official scheduling alternative

`claude-subscription` remains fail-closed because this project has not confirmed a minimal local Claude subscription execution contract that meets its safety requirements. Do not replace it with an API-key or `--bare` fallback. Claude Code's official alternatives are separate products: `/schedule` creates a persistent cloud Routine, Claude Desktop can create a persistent local scheduled task, and `/loop` runs only while a CLI session is open (or restored while unexpired). Choose those directly when their execution location, repository access, permissions, and persistence model fit the task; they are not `open-agent-clock` bindings.

## Preview and run

Preview a target without starting a provider process:

```bash
open-agent-clock run --dry-run --target native-codex --prompt hi
open-agent-clock run --dry-run --target hermes-codex --prompt hi --json
```

Run one explicitly selected schedule with the normal scheduled invocation path:

```bash
open-agent-clock run --once --schedule codex-window --confirm
```

For a real, explicitly confirmed development/diagnostic run only, add `--dev` (or its `--diagnostic` alias). Native Codex then receives JSONL mode so provider-reported usage can be parsed; the normal command above and `launchd`/`tick` invocations do not receive `--json`:

```bash
open-agent-clock run --once --schedule codex-window --confirm --dev
open-agent-clock run --once --schedule codex-window --confirm --diagnostic
open-agent-clock last-run --schedule codex-window --dev
```

For native Codex, the dry-run command is planned as `codex --ask-for-approval never exec --ephemeral --sandbox read-only --skip-git-repo-check hi`; dev mode adds `--json` immediately before `hi`. The trusted-directory flag is required because provider processes run in a temporary empty working directory.

Foreground/debug tick:

```bash
open-agent-clock tick --schedule codex-window --confirm
```

Every real provider invocation is bounded by a timeout and output limit, uses an isolated temporary working directory, and does not automatically retry usage-limit failures.

## Telemetry and diagnostics

Token counters are recorded only when the provider adapter receives structured provider-reported usage metadata. The fields are labeled `provider-reported`; `availability: unavailable` means the provider did not expose a supported usage envelope. Native Codex dev JSONL usage includes `input_tokens`, optional `cached_input_tokens`, `output_tokens`, and `total_tokens`; the tool never estimates tokens from prompt length, characters, words, or local timing.

Inspect structured data for automation, or explicitly request bounded human diagnostics:

```bash
open-agent-clock history --json
open-agent-clock history --diagnostic
open-agent-clock last-run --schedule codex-window --json
open-agent-clock last-run --schedule codex-window --diagnostic
```

Diagnostics use stable categories: `auth`, `quota`, `network`, `arguments`, `provider`, and `unknown`. Details are short, redacted, and bounded. The application never persists or renders complete prompts, responses, credentials, or raw/unbounded provider stdout/stderr. Legacy history and state records load successfully and show token usage as unavailable.

## macOS launchd

Preview the LaunchAgent without writing files or loading jobs:

```bash
open-agent-clock schedule install --id codex-window --dry-run
```

Install and inspect a user-level LaunchAgent:

```bash
open-agent-clock schedule install --id codex-window --confirm
open-agent-clock schedule status
```

Remove it:

```bash
open-agent-clock schedule uninstall --id codex-window --confirm
```

The tool manages only its own labels with the `com.openagentclock.schedule.*` prefix. Root privileges are not required.

## Inspect and manage

```bash
open-agent-clock status
open-agent-clock status --json
open-agent-clock schedule list
open-agent-clock schedule update --id daily-check --times 06:00,14:00
open-agent-clock pause --id codex-window
open-agent-clock resume --id codex-window --confirm
open-agent-clock history
open-agent-clock history --json
open-agent-clock last-run --schedule codex-window
```

History stores redacted metadata only: timestamps, duration, status, exit code, and provider version. Provider credentials and full provider output are not stored by the application.

## Safety model

- This tool is **not a limit bypass or resetter**.
- A successful local exit code does not prove that a server-side usage window changed.
- **API-key targets do not participate in usage-window schedules.**
- The application does not copy, persist, print, or log provider credentials.
- Real Codex, Hermes, or Claude calls do not happen during detection, initialization, schedule creation, or dry-run.
- Real calls require an explicit target/schedule and `--confirm`.
- One target's failure does not start, retry, or alter another target.

## Data locations

On macOS:

```text
~/Library/Application Support/open-agent-clock/
```

The directory contains local configuration, state, history, and generated runtime files. Files are created with user-only permissions where applicable.

## Uninstall

First unload every managed schedule shown by `open-agent-clock schedule status`:

```bash
open-agent-clock schedule uninstall --id <schedule-id> --confirm
```

Then remove the installed binary. Remove `~/Library/Application Support/open-agent-clock/` separately only if you also want to discard local configuration and redacted history. The tool never removes unrelated LaunchAgents.

## Troubleshooting

- **Command not found:** add `~/.local/bin` (or `$(go env GOPATH)/bin` for `go install`) to `PATH`.
- **No targets detected:** sign in through the provider's official CLI first, then run `open-agent-clock detect`; the tool never asks for or copies credentials.
- **Schedule is not running:** check `open-agent-clock status`, `open-agent-clock schedule status`, and `open-agent-clock last-run --schedule <id>`; verify that the schedule is enabled and its managed LaunchAgent is loaded.
- **A run was skipped:** overlapping invocations and missed polling windows are intentionally skipped rather than caught up or retried.
- **Provider rejected the request:** inspect the redacted status/history and use the provider CLI directly for account diagnostics. A scheduled local invocation does not guarantee capacity or a server-side usage-window reset.
- **Timezone behavior is unexpected:** verify the configured IANA timezone. Daily schedules outside the Mac's system timezone use one-minute timezone-aware polling.

## Development

```bash
make fmt
make test
make vet
make build
```

The project also runs formatting, vet, tests, and build in GitHub Actions.

## Project status

The MVP is implemented and under active hardening. Native Codex and Hermes execution paths, schedules, history, dry-run, locking, and guarded LaunchAgent integration are available. Real provider smoke tests are intentionally not run automatically because they may consume subscription allowance.

Future ideas that are not scheduled for implementation are tracked in [`ROADMAP.md`](ROADMAP.md).

## References

- [OpenAI Codex non-interactive CLI](https://developers.openai.com/codex/noninteractive)
- [OpenAI Codex CLI reference](https://developers.openai.com/codex/cli/reference)
- [Claude Code headless mode](https://docs.anthropic.com/en/docs/claude-code/headless)
- [Claude Code scheduled tasks and `/schedule`](https://code.claude.com/docs/en/scheduled-tasks)
- [GitHub: About repository README files](https://docs.github.com/en/repositories/creating-and-managing-repositories/customizing-your-repository/about-readmes)
