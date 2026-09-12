#!/bin/sh
set -eu

REPOSITORY="${OPEN_AGENT_CLOCK_REPOSITORY:-AlekseyBeketov/open-agent-clock}"
INSTALL_DIR="${OPEN_AGENT_CLOCK_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${OPEN_AGENT_CLOCK_VERSION:-latest}"
RUN_SETUP=1

usage() {
  cat <<'EOF'
Install open-agent-clock on macOS and open the guided terminal setup.

Usage: install.sh [--no-setup]

Environment:
  OPEN_AGENT_CLOCK_INSTALL_DIR  Destination directory (default: ~/.local/bin)
  OPEN_AGENT_CLOCK_VERSION      Release tag or "latest" (default: latest)
  OPEN_AGENT_CLOCK_REPOSITORY   GitHub owner/repository override
EOF
}

for argument in "$@"; do
  case "$argument" in
    --no-setup) RUN_SETUP=0 ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'Unknown option: %s\n' "$argument" >&2; usage >&2; exit 2 ;;
  esac
done

if [ "$(uname -s)" != "Darwin" ]; then
  printf '%s\n' 'open-agent-clock currently supports macOS only.' >&2
  exit 1
fi

case "$(uname -m)" in
  arm64) ARCH="arm64" ;;
  x86_64) ARCH="amd64" ;;
  *) printf 'Unsupported macOS architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/open-agent-clock-install.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM
BINARY="$TMP_DIR/open-agent-clock"

install_local_binary() {
  cp "$OPEN_AGENT_CLOCK_BINARY" "$BINARY"
}

download_release() {
  command -v curl >/dev/null 2>&1 || return 1
  archive="open-agent-clock_darwin_${ARCH}.tar.gz"
  if [ "$VERSION" = "latest" ]; then
    base="https://github.com/${REPOSITORY}/releases/latest/download"
  else
    base="https://github.com/${REPOSITORY}/releases/download/${VERSION}"
  fi

  printf 'Downloading %s...\n' "$archive"
  http_code="$(curl -sS -L -w '%{http_code}' "$base/$archive" -o "$TMP_DIR/$archive")" || {
    printf '%s\n' 'Release download failed; refusing to fall back after a network error.' >&2
    return 1
  }
  if [ "$http_code" = "404" ]; then
    return 2
  fi
  [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ] || {
    printf 'Release download returned HTTP %s; refusing to install.\n' "$http_code" >&2
    return 1
  }
  curl -fsSL "$base/checksums.txt" -o "$TMP_DIR/checksums.txt" || {
    printf '%s\n' 'Checksum download failed; refusing to install.' >&2
    return 1
  }

  expected="$(awk -v file="$archive" '$2 == file || $2 == "*" file { print $1; exit }' "$TMP_DIR/checksums.txt")"
  [ -n "$expected" ] || {
    printf 'Checksum entry for %s was not found.\n' "$archive" >&2
    return 1
  }
  actual="$(shasum -a 256 "$TMP_DIR/$archive" | awk '{print $1}')"
  [ "$actual" = "$expected" ] || {
    printf '%s\n' 'Checksum verification failed; refusing to install.' >&2
    return 1
  }

  validate_archive "$TMP_DIR/$archive" || return 1
  tar -xzf "$TMP_DIR/$archive" -C "$TMP_DIR"
  [ -x "$BINARY" ]
}

build_with_go() {
  command -v go >/dev/null 2>&1 || return 1
  printf '%s\n' 'No release asset is available; building the current version with Go...'
  mkdir -p "$TMP_DIR/go-bin"
  source_ref="main"
  if [ "$VERSION" != "latest" ]; then
    source_ref="$VERSION"
  fi
  GOBIN="$TMP_DIR/go-bin" go install "github.com/${REPOSITORY}/cmd/open-agent-clock@${source_ref}"
  cp "$TMP_DIR/go-bin/open-agent-clock" "$BINARY"
}

validate_archive() {
  archive_path="$1"
  entries_file="$TMP_DIR/archive.entries"
  details_file="$TMP_DIR/archive.details"
  tar -tzf "$archive_path" > "$entries_file" || return 1
  while IFS= read -r entry; do
    case "$entry" in
      /*|../*|*/../*|*/..|..)
        printf 'Unsafe archive path: %s\n' "$entry" >&2
        return 1
        ;;
    esac
  done < "$entries_file"
  tar -tvzf "$archive_path" > "$details_file" || return 1
  while IFS= read -r entry; do
    case "$entry" in
      l*|h*)
        printf '%s\n' 'Symlink and hardlink archive entries are not allowed.' >&2
        return 1
        ;;
    esac
  done < "$details_file"
}

if [ -n "${OPEN_AGENT_CLOCK_BINARY:-}" ]; then
  install_local_binary
else
  if download_release; then
    :
  else
    download_status=$?
    if [ "$download_status" -ne 2 ]; then
      exit 1
    fi
    build_with_go || {
      printf '%s\n' 'No release asset is available. Install Go 1.27+ or download a release binary manually.' >&2
      exit 1
    }
  fi
fi

mkdir -p "$INSTALL_DIR"
chmod 755 "$BINARY"
cp "$BINARY" "$INSTALL_DIR/open-agent-clock"
chmod 755 "$INSTALL_DIR/open-agent-clock"

printf 'Installed: %s\n' "$INSTALL_DIR/open-agent-clock"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) printf 'Add this directory to PATH: export PATH="%s:$PATH"\n' "$INSTALL_DIR" ;;
esac

if [ "$RUN_SETUP" -eq 1 ] && [ -r /dev/tty ] && [ -w /dev/tty ]; then
  exec "$INSTALL_DIR/open-agent-clock" setup </dev/tty >/dev/tty 2>/dev/tty
fi

printf 'Start guided setup later with: %s setup\n' "$INSTALL_DIR/open-agent-clock"
