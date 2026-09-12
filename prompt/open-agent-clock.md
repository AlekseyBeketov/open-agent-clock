Настрой прозрачное локальное расписание запусков официальных terminal CLI в `open-agent-clock`.

Обязательные правила:

- Это не обходчик и не сбросчик лимитов.
- API-key targets не участвуют в usage-window schedules.
- Не копируй, не печатай и не сохраняй credentials.
- Сначала выполни detection и dry-run.
- Реальные вызовы Codex/Claude до этого запускаться не будут без отдельного явного подтверждения владельца.
- Предупреди, что запуск может расходовать subscription allowance, завершиться ошибкой или быть отклонён провайдером и не гарантирует server-side reset time или дополнительную capacity.

Порядок:

1. Обнаружь `native-codex` и `hermes-codex` независимо.
2. Не утверждай, что это разные accounts, если identity не подтверждена официальным неперсональным признаком.
3. Создай disabled schedule.
4. Покажи `run --dry-run`, target, backend, auth mode, cwd policy, timezone и next run.
5. Для включения потребуй explicit `--confirm`.
6. Для launchd сначала покажи `schedule install --dry-run`, затем устанавливай только выбранный user-level LaunchAgent.
7. Используй `status`, `history`, `last-run` и `schedule status` для диагностики.
8. При usage/rate limit не делай automatic retry и не называй результат reset.
9. Для Claude не используй API-key/`--bare` fallback в usage-window scheduling; если subscription-compatible local path не подтверждён, оставь capability unsupported и упомяни официальный provider-native `/schedule` как отдельную альтернативу.

Defaults: interval `5h3m`, prompt `hi`, morning example `05:00`, missed runs skip.
