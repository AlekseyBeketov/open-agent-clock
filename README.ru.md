# open-agent-clock

[English](README.md)

[![CI](https://github.com/AlekseyBeketov/open-agent-clock/actions/workflows/ci.yml/badge.svg)](https://github.com/AlekseyBeketov/open-agent-clock/actions/workflows/ci.yml)

**Прозрачный macOS-планировщик для официальных agent CLI.**

`open-agent-clock` планирует явно разрешённые локальные запуски установленных terminal agents — например, Codex CLI и Hermes. Он помогает привязать повторяющуюся работу к собственному расписанию, сохраняя execution path видимым, проверяемым и управляемым пользователем.

> Это **не обходчик и не сбросчик лимитов**. Инструмент не обещает server-side reset, дополнительную capacity или успешное принятие запроса провайдером.

## Зачем нужен open-agent-clock?

- Независимо планировать разные локальные targets.
- До запуска увидеть точную команду.
- Использовать macOS `launchd` без daemon и root privileges.
- Оставлять credentials в auth store самого провайдера.
- Сохранять redacted metadata для диагностики.
- Работать fail-closed: реальный запуск и активация расписания требуют явного подтверждения.

## Поддерживаемые targets

| Target | Provider | Локальный execution path | Usage-window scheduling |
|---|---|---|---|
| `native-codex` | OpenAI Codex | Нативный `codex` CLI и `~/.codex` | Только subscription |
| `hermes-codex` | OpenAI Codex | Hermes CLI с `openai-codex` | Только subscription |
| `claude-subscription` | Claude | Обнаруживается только при безопасном подтверждении subscription/OAuth path | Только subscription |

Native Codex и Hermes намеренно представлены как разные targets. Два локальных credential stores не доказывают использование двух разных server accounts; identity может оставаться `unknown`.

**API-key targets не участвуют в usage-window schedules.**

## Prompt и Skill для agent

CLI работает самостоятельно. В репозитории также есть два необязательных файла для пользователей, которые хотят поручить безопасную настройку или управление AI coding agent:

Оба artifact написаны на английском для переносимости, но явно требуют от agent общаться на привычном или выбранном пользователем языке.

> **Открыть canonical prompt:** [`prompt/open-agent-clock.md`](prompt/open-agent-clock.md). Полный текст намеренно находится в отдельном файле и не занимает экран README.

- [`skill/open-agent-clock/SKILL.md`](skill/open-agent-clock/SKILL.md) — переиспользуемый agent skill для detection, dry-run, активации расписания, управления LaunchAgent и диагностики.

Сначала просмотрите prompt, затем передайте его agent:

```bash
curl -fsSL https://raw.githubusercontent.com/AlekseyBeketov/open-agent-clock/main/prompt/open-agent-clock.md
```

Установить skill в стандартный каталог agent skills:

```bash
mkdir -p ~/.agents/skills/open-agent-clock
curl -fsSL https://raw.githubusercontent.com/AlekseyBeketov/open-agent-clock/main/skill/open-agent-clock/SKILL.md \
  -o ~/.agents/skills/open-agent-clock/SKILL.md
```

Если ваш agent использует другой каталог skills, укажите его — например, `~/.codex/skills/open-agent-clock/` для Codex CLI или `~/.hermes/skills/open-agent-clock/` для Hermes. Перед установкой прочитайте файл так же, как вы проверяете `install.sh`.

### Границы context и token usage

`open-agent-clock` минимизирует **локальный invocation context**: default prompt содержит только `hi`, provider processes запускаются во временной пустой рабочей директории, native Codex получает `--ephemeral`, когда установленный CLI поддерживает этот flag, а Hermes работает в one-shot режиме без project rules и optional toolsets. Поэтому вызов, построенный инструментом, не прикладывает этот repository, предыдущую interactive conversation или несвязанные agent skills.

Инструмент **не** сбрасывает и не стирает встроенный system prompt provider CLI, provider-managed metadata или историю вне гарантий документированных flags этого CLI. Он также не может сбросить usage window или гарантировать конкретное уменьшение context window либо billed tokens. Команда `run --dry-run` показывает ровно ту часть вызова, которую контролирует инструмент.

## Установка

### Рекомендуемый способ: одна команда

На macOS installer определяет архитектуру, проверяет checksum релиза, устанавливает бинарник в `~/.local/bin` и открывает пошаговую настройку:

```bash
curl -fsSL https://raw.githubusercontent.com/AlekseyBeketov/open-agent-clock/main/install.sh | sh
```

Если этого требует ваша security policy, сначала прочитайте [`install.sh`](install.sh). Установить приложение без запуска setup:

```bash
curl -fsSL https://raw.githubusercontent.com/AlekseyBeketov/open-agent-clock/main/install.sh | sh -s -- --no-setup
```

### Установка через Go

Требуются macOS и Go 1.27+:

```bash
go install github.com/AlekseyBeketov/open-agent-clock/cmd/open-agent-clock@latest
```

Открыть пошаговую настройку:

```bash
open-agent-clock setup
```

Если shell не находит команду, добавьте Go bin directory в `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

### Сборка из clone

```bash
git clone https://github.com/AlekseyBeketov/open-agent-clock.git && cd open-agent-clock && make build
```

Бинарник появится в `bin/open-agent-clock`.

### Запуск из checkout без установки

```bash
go run ./cmd/open-agent-clock detect
```

Опубликованные releases содержат checksum-verified archives для macOS arm64 и amd64. Если запрошенного release asset нет, installer может собрать приложение из source при наличии Go 1.27+. Package-manager distribution запланирована позднее.

## Обновления

Проверить release metadata без изменения бинарника, посмотреть состояние auto-update или явно установить update:

```bash
open-agent-clock update check
open-agent-clock update status
open-agent-clock update install --confirm
open-agent-clock update disable
```

Interactive setup и **Settings → Automatic updates** предлагают opt-in после создания interval schedule. Включение привязывает одну попытку обновления к существующим cadence и phase этого расписания; второй timer не создаётся. Непосредственно перед подтверждённым due tick инструмент выполняет не более одной bounded attempt, сохраняет non-secret result и продолжает provider tick даже при ошибке обновления. По умолчанию обновления выключены, включая migrated configurations.

Установка разрешена только для released semantic versions и напрямую управляемых writable macOS binaries. Updater выбирает точный asset для `darwin` architecture, обязательно требует `checksums.txt`, проверяет SHA-256 до extraction, отклоняет links и unsafe archive paths, создаёт staged executable в том же каталоге и заменяет бинарник атомарно. Development builds, package-manager paths, отсутствие checksums, повреждённые archives, network failures и unwritable directories завершаются fail-closed без замены рабочего бинарника. `sudo` не используется, retry loop внутри одного occurrence не выполняется.

## macOS-уведомления о завершении

Guided setup и **Settings → Completion notifications** предлагают явный opt-in; по умолчанию уведомления выключены, включая migrated configurations. Отключить их можно позднее через тот же пункт Settings.

После завершения реального provider attempt инструмент делает не более одного bounded best-effort запроса через встроенный macOS `/usr/bin/osascript`. Уведомление содержит только ID управляемого расписания, target ID и classification `success`/`failure`. В нём никогда нет prompt, credentials, provider stdout/stderr или diagnostic reason. macOS может запросить разрешение либо подавить доставку согласно System Settings и Focus. Ошибка доставки выводится в stderr, но не меняет сохранённый provider result и никогда не повторяет provider invocation.

## Быстрый старт: пошаговая настройка в терминале

Запустите приложение одной из команд:

```bash
open-agent-clock
# или явно:
open-agent-clock setup
```

На первом экране выбирается язык, English установлен по умолчанию. Затем wizard обнаруживает локально авторизованные subscription targets и помогает настроить режим, время, timezone, prompt, preview, активацию и необязательную установку LaunchAgent. Он не запрашивает credentials и создаёт расписание выключенным до отдельного подтверждения активации.

После setup запускайте `open-agent-clock` или `open-agent-clock ui`: через меню доступны targets, schedules, previews, запуски, LaunchAgents, status, history и settings.

### Доступный line-oriented режим

Для screen readers и терминалов, в которых full-screen формы работают ненадёжно:

```bash
ACCESSIBLE=1 open-agent-clock setup
ACCESSIBLE=1 open-agent-clock ui
```

### Прямые команды для automation

Все операции по-прежнему доступны как non-interactive commands. Например:

```bash
open-agent-clock init
open-agent-clock detect
open-agent-clock schedule add --id codex-window --target native-codex --mode interval --interval 5h3m --timezone Europe/Moscow
open-agent-clock run --dry-run --target native-codex --prompt hi
open-agent-clock resume --id codex-window --confirm
open-agent-clock schedule install --id codex-window --confirm
```

`schedule add` по умолчанию создаёт выключенное расписание. `--confirm` обязателен при включении provider schedule и загрузке LaunchAgent. Запуск без аргументов в non-interactive context печатает help и никогда не ожидает TUI input.

## Режимы расписания

### Interval

Повторять запуск через заданный интервал. Default interval — `5h3m`:

```bash
open-agent-clock schedule add \
  --id codex-window \
  --target native-codex \
  --mode interval \
  --interval 5h3m \
  --timezone Europe/Moscow
```

### Morning

Один запуск в день в выбранное локальное время:

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

Расписания учитывают timezone и используют IANA timezone names. Пропущенные `launchd`-запуски пропускаются и автоматически не догоняются.

### Materialization trigger в launchd и ограничения

- Interval schedules материализуются как минутный polling trigger через `StartInterval`. Команда `tick` применяет заданный interval от начала расписания или последнего завершённого запуска, поэтому загрузка LaunchAgent не сдвигает anchor интервала.
- Morning/custom daily schedules используют native `StartCalendarInterval`, когда их IANA timezone совпадает с системным timezone macOS.
- У `StartCalendarInterval` нет поля timezone. Для daily schedule в другом IANA timezone CLI использует тот же минутный polling trigger и сам вычисляет заданный timezone, вместо тихого запуска в неправильное локальное время.
- Interval короче одной минуты нельзя материализовать для launchd: `schedule install` отклонит его. Один plist не может одновременно содержать interval и calendar triggers.

### Официальная альтернатива scheduling в Claude

`claude-subscription` остаётся fail-closed: проект пока не подтвердил minimal local execution contract для Claude subscription, соответствующий его требованиям безопасности. Нельзя подменять его API-key или `--bare` fallback. Официальные варианты Claude Code являются отдельными механизмами: `/schedule` создаёт persistent cloud Routine, Claude Desktop — persistent local scheduled task, а `/loop` работает только пока открыта CLI session (либо она восстановлена до истечения задачи). Выбирайте их напрямую, если подходят execution location, доступ к repository, permissions и persistence; они не являются bindings `open-agent-clock`.

## Preview и запуск

Preview target без запуска provider process:

```bash
open-agent-clock run --dry-run --target native-codex --prompt hi
open-agent-clock run --dry-run --target hermes-codex --prompt hi --json
```

Однократный запуск конкретного расписания через обычный scheduled invocation path:

```bash
open-agent-clock run --once --schedule codex-window --confirm
```

Только для реального явно подтверждённого development/diagnostic запуска добавьте `--dev` (или alias `--diagnostic`). Тогда native Codex получает JSONL mode, и adapter может извлечь provider-reported usage; обычная команда выше и invocations через `launchd`/`tick` не получают `--json`:

```bash
open-agent-clock run --once --schedule codex-window --confirm --dev
open-agent-clock run --once --schedule codex-window --confirm --diagnostic
open-agent-clock last-run --schedule codex-window --dev
```

Для native Codex dry-run планирует точную команду `codex --ask-for-approval never exec --ephemeral --sandbox read-only --skip-git-repo-check hi`; dev mode добавляет `--json` непосредственно перед `hi`. Trusted-directory flag нужен, потому что provider process запускается во временной пустой рабочей директории.

Foreground/debug tick:

```bash
open-agent-clock tick --schedule codex-window --confirm
```

Каждый реальный provider invocation ограничен timeout и output limit, запускается во временной изолированной рабочей директории и не повторяется автоматически после usage-limit error.

## Telemetry и диагностика

Счётчики токенов сохраняются только если adapter провайдера получил structured provider-reported usage metadata. Поле помечено `provider-reported`; `availability: unavailable` означает, что provider не предоставил поддерживаемый usage envelope. Native Codex dev JSONL usage включает `input_tokens`, необязательный `cached_input_tokens`, `output_tokens` и `total_tokens`. Инструмент никогда не оценивает токены по длине prompt, символам, словам или local timing.

Для automation используйте structured JSON, а для человека явно включайте bounded diagnostics:

```bash
open-agent-clock history --json
open-agent-clock history --diagnostic
open-agent-clock last-run --schedule codex-window --json
open-agent-clock last-run --schedule codex-window --diagnostic
```

Диагностика использует стабильные категории: `auth`, `quota`, `network`, `arguments`, `provider` и `unknown`. Detail короткий, redacted и bounded. Приложение не сохраняет и не выводит полные prompts, responses, credentials или raw/unbounded provider stdout/stderr. Legacy history и state загружаются успешно, а token usage показывается как unavailable.

## macOS launchd

Preview LaunchAgent без записи файлов и загрузки job:

```bash
open-agent-clock schedule install --id codex-window --dry-run
```

Установить и проверить user-level LaunchAgent:

```bash
open-agent-clock schedule install --id codex-window --confirm
open-agent-clock schedule status
```

Удалить его:

```bash
open-agent-clock schedule uninstall --id codex-window --confirm
```

Инструмент управляет только собственными labels с префиксом `com.openagentclock.schedule.*`. Root privileges не нужны.

## Проверка и управление

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

History хранит только redacted metadata: timestamps, duration, status, exit code и provider version. Credentials и полный provider output приложение не сохраняет.

## Модель безопасности

- Инструмент **не является обходчиком или сбросчиком лимитов**.
- Успешный local exit code не доказывает изменение server-side usage window.
- **API-key targets не участвуют в usage-window schedules.**
- Приложение не копирует, не сохраняет, не выводит и не логирует credentials провайдеров.
- Во время detection, initialization, создания расписания и dry-run реальные вызовы Codex, Hermes или Claude не выполняются.
- Реальные вызовы требуют явно выбранный target/schedule и `--confirm`.
- Ошибка одного target не запускает, не повторяет и не изменяет другой target.

## Где хранятся данные

На macOS:

```text
~/Library/Application Support/open-agent-clock/
```

В каталоге находятся локальные configuration, state, history и generated runtime files. Где применимо, файлы создаются с user-only permissions.

## Удаление

Сначала выгрузите каждое управляемое расписание из `open-agent-clock schedule status`:

```bash
open-agent-clock schedule uninstall --id <schedule-id> --confirm
```

Затем удалите установленный бинарник. Удаляйте `~/Library/Application Support/open-agent-clock/` отдельно, только если хотите также стереть локальную configuration и redacted history. Инструмент не удаляет чужие LaunchAgents.

## Решение проблем

- **Команда не найдена:** добавьте `~/.local/bin` (или `$(go env GOPATH)/bin` при `go install`) в `PATH`.
- **Targets не обнаружены:** сначала авторизуйтесь через официальный CLI провайдера, затем запустите `open-agent-clock detect`; инструмент не запрашивает и не копирует credentials.
- **Расписание не запускается:** проверьте `open-agent-clock status`, `open-agent-clock schedule status` и `open-agent-clock last-run --schedule <id>`; убедитесь, что расписание включено, а его управляемый LaunchAgent загружен.
- **Запуск пропущен:** пересекающиеся invocations и пропущенные polling windows намеренно пропускаются без catch-up и retry.
- **Провайдер отклонил запрос:** изучите redacted status/history и используйте CLI провайдера для диагностики аккаунта. Локальный scheduled invocation не гарантирует capacity или server-side reset окна использования.
- **Неожиданное поведение timezone:** проверьте настроенную IANA timezone. Daily schedules вне системной timezone Mac используют timezone-aware polling раз в минуту.

## Разработка

```bash
make fmt
make test
make vet
make build
```

GitHub Actions также выполняет formatting, vet, tests и build.

## Статус проекта

MVP реализован и проходит active hardening. Доступны native Codex и Hermes execution paths, расписания, history, dry-run, locking и guarded LaunchAgent integration. Реальные provider smoke tests намеренно не запускаются автоматически, поскольку они могут расходовать subscription allowance.

Будущие идеи, пока не запланированные к реализации, записаны в [`ROADMAP.md`](ROADMAP.md).

## Ссылки

- [OpenAI Codex non-interactive CLI](https://developers.openai.com/codex/noninteractive)
- [OpenAI Codex CLI reference](https://developers.openai.com/codex/cli/reference)
- [Claude Code headless mode](https://docs.anthropic.com/en/docs/claude-code/headless)
- [Claude Code scheduled tasks и `/schedule`](https://code.claude.com/docs/en/scheduled-tasks)
- [GitHub: About repository README files](https://docs.github.com/en/repositories/creating-and-managing-repositories/customizing-your-repository/about-readmes)
