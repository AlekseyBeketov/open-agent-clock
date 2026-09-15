#!/bin/sh
if [ "$1" = "--version" ]; then
  printf 'fake-provider 0.0.1\n'
  exit 0
fi
if [ "$1" = "exec" ] && [ "$2" = "--help" ]; then
  printf 'Usage: codex exec [OPTIONS] --ephemeral --sandbox <MODE> --skip-git-repo-check --json\n'
  exit 0
fi
if [ "$1" = "--ask-for-approval" ]; then
  if [ "$2" != "never" ] || [ "$3" != "exec" ] || [ "$4" != "--ephemeral" ] || [ "$5" != "--sandbox" ] || [ "$6" != "read-only" ] || [ "$7" != "--skip-git-repo-check" ]; then
    printf 'unexpected native Codex arguments\n' >&2
    exit 64
  fi
  case "${FAKE_PROVIDER_SCENARIO-}" in
    success-usage)
      if [ "$8" = "--json" ] && [ -n "$9" ]; then
        printf '{"type":"turn.completed","usage":{"input_tokens":11,"cached_input_tokens":5,"output_tokens":7,"total_tokens":18}}\n'
      elif [ -n "$8" ] && [ -z "$9" ]; then
        printf 'fake native Codex completed\n'
      else
        printf 'unexpected native Codex dev arguments\n' >&2
        exit 64
      fi
      exit 0
      ;;
    unavailable)
      printf 'fake provider completed without structured usage\n'
      exit 0
      ;;
    quota)
      printf 'quota exceeded: rate limit reached\n' >&2
      exit 1
      ;;
    auth)
      printf 'authentication required: token expired\n' >&2
      exit 1
      ;;
    arguments)
      printf 'unknown option: --not-a-real-flag\n' >&2
      exit 1
      ;;
    network)
      printf 'network connection refused\n' >&2
      exit 1
      ;;
    provider)
      printf 'provider internal failure authorization=synthetic-provider-secret\n' >&2
      exit 1
      ;;
  esac
  if [ -n "$8" ] && [ -z "$9" ]; then
    printf 'fake native Codex completed\n'
    exit 0
  fi
  printf 'unexpected native Codex arguments\n' >&2
  exit 64
fi
if [ "$1" = "chat" ]; then
  if [ "$2" != "--provider" ] || [ "$3" != "openai-codex" ] || [ "$4" != "--safe-mode" ] || [ "$5" != "--ignore-rules" ] || [ "$6" != "--toolsets" ] || [ -n "$7" ] || [ "$8" != "--oneshot" ] || [ "$9" != "--quiet" ] || [ "${10}" != "-q" ] || [ -z "${11}" ]; then
    printf 'unexpected Hermes arguments\n' >&2
    exit 64
  fi
  printf 'fake Hermes Codex completed\n'
  exit 0
fi
printf 'unexpected fake provider invocation\n' >&2
exit 64
