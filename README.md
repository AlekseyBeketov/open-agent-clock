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

- [`prompt/open-agent-clock.md`](prompt/open-agent-clock.md) — a ready-to-paste setup prompt with the project's consent and credential boundaries.
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

## Preview and run

Preview a target without starting a provider process:

```bash
open-agent-clock run --dry-run --target native-codex --prompt hi
open-agent-clock run --dry-run --target hermes-codex --prompt hi --json
```

Run one explicitly selected schedule:

```bash
open-agent-clock run --once --schedule codex-window --confirm
```

Foreground/debug tick:

```bash
open-agent-clock tick --schedule codex-window --confirm
```

Every real provider invocation is bounded by a timeout and output limit, uses an isolated temporary working directory, and does not automatically retry usage-limit failures.

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

## References

- [OpenAI Codex non-interactive CLI](https://developers.openai.com/codex/noninteractive)
- [OpenAI Codex CLI reference](https://developers.openai.com/codex/cli/reference)
- [Claude Code headless mode](https://docs.anthropic.com/en/docs/claude-code/headless)
- [GitHub: About repository README files](https://docs.github.com/en/repositories/creating-and-managing-repositories/customizing-your-repository/about-readmes)
