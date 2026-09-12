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

- [`prompt/open-agent-clock.md`](prompt/open-agent-clock.md) — готовый setup prompt с правилами consent, credentials и dry-run.
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

## Preview и запуск

Preview target без запуска provider process:

```bash
open-agent-clock run --dry-run --target native-codex --prompt hi
open-agent-clock run --dry-run --target hermes-codex --prompt hi --json
```

Однократный запуск конкретного расписания:

```bash
open-agent-clock run --once --schedule codex-window --confirm
```

Foreground/debug tick:

```bash
open-agent-clock tick --schedule codex-window --confirm
```

Каждый реальный provider invocation ограничен timeout и output limit, запускается во временной изолированной рабочей директории и не повторяется автоматически после usage-limit error.

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

## Ссылки

- [OpenAI Codex non-interactive CLI](https://developers.openai.com/codex/noninteractive)
- [OpenAI Codex CLI reference](https://developers.openai.com/codex/cli/reference)
- [Claude Code headless mode](https://docs.anthropic.com/en/docs/claude-code/headless)
- [GitHub: About repository README files](https://docs.github.com/en/repositories/creating-and-managing-repositories/customizing-your-repository/about-readmes)
