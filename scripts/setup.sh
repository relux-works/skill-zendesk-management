#!/usr/bin/env zsh

set -euo pipefail

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BINARY_NAME="zendesk-mgmt"
BUILD_OUTPUT="$SKILL_DIR/$BINARY_NAME"
BIN_DIR="${ZENDESK_MGMT_BIN_DIR:-$HOME/.local/bin}"
INSTALL_ONLY="${ZENDESK_MGMT_INSTALL_ONLY:-0}"
CONFIG_DIR="${ZENDESK_MGMT_CONFIG_DIR:-$HOME/Library/Application Support/zendesk-mgmt}"
INSTALL_STATE_PATH="$CONFIG_DIR/install.json"
AGENTS_DEST="$HOME/.agents/skills/zendesk-management"
CLAUDE_DEST="$HOME/.claude/skills/zendesk-management"
CODEX_DEST="$HOME/.codex/skills/zendesk-management"
BUILD_VERSION="dev"
BUILD_COMMIT="unknown"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
BUILD_LDFLAGS=""

green() { print -P "%F{green}$1%f"; }
yellow() { print -P "%F{yellow}$1%f"; }
red() { print -P "%F{red}$1%f"; }

json_escape() {
  print -rn -- "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

usage() {
  cat <<EOF
Usage: scripts/setup.sh [options]

Options:
  --bin-dir PATH       Install binary into PATH (default: $HOME/.local/bin)
  --install-only       Safe reinstall of binary, skill artifact, links, and install state
  --help, -h           Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin-dir)
      BIN_DIR="$2"
      shift 2
      ;;
    --install-only)
      INSTALL_ONLY="1"
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      red "Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

install_go() {
  if command -v go >/dev/null 2>&1; then
    green "Go already installed: $(go version)"
    return
  fi

  if ! command -v brew >/dev/null 2>&1; then
    red "Go is missing and Homebrew is not available. Install Homebrew or Go first."
    exit 1
  fi

  yellow "Go not found. Installing via Homebrew..."
  brew install go
  green "Go installed: $(go version)"
}

compute_ldflags() {
  if git -C "$SKILL_DIR" rev-parse --git-dir >/dev/null 2>&1; then
    BUILD_VERSION="$(git -C "$SKILL_DIR" describe --tags --always 2>/dev/null || echo "dev")"
    BUILD_COMMIT="$(git -C "$SKILL_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")"
  fi

  BUILD_LDFLAGS="-X main.Version=$BUILD_VERSION -X main.Commit=$BUILD_COMMIT -X main.BuildDate=$BUILD_DATE"
}

build_cli() {
  green "Building $BINARY_NAME ..."
  (
    cd "$SKILL_DIR"
    go build -trimpath -ldflags "$BUILD_LDFLAGS" -o "$BUILD_OUTPUT" ./cmd/zendesk-mgmt
  )
  green "Built: $BUILD_OUTPUT"
}

install_binary() {
  local dest="$BIN_DIR/$BINARY_NAME"
  mkdir -p "$BIN_DIR"
  cp "$BUILD_OUTPUT" "$dest"
  chmod +x "$dest"
  green "Installed binary: $dest"
}

scrub_git_metadata() {
  local dir="$1"
  for rel in .git .gitignore .gitattributes .gitmodules; do
    rm -rf "$dir/$rel" 2>/dev/null || true
  done
}

install_skill_artifact() {
  mkdir -p "$AGENTS_DEST"
  rsync -a --delete "$SKILL_DIR/" "$AGENTS_DEST/" \
    --exclude='.git' \
    --exclude='.task-board' \
    --exclude='.agents' \
    --exclude='.claude' \
    --exclude='.codex' \
    --exclude='.local' \
    --exclude='dist' \
    --exclude='zendesk-mgmt' \
    --exclude='zendesk-mgmt.exe'
  scrub_git_metadata "$AGENTS_DEST"
  green "Installed skill artifact: $AGENTS_DEST"
}

refresh_links() {
  mkdir -p "$HOME/.claude/skills" "$HOME/.codex/skills"
  rm -rf "$CLAUDE_DEST" "$CODEX_DEST"
  ln -s "$AGENTS_DEST" "$CLAUDE_DEST"
  ln -s "$AGENTS_DEST" "$CODEX_DEST"
  green "Refreshed Claude/Codex skill links"
}

write_install_state() {
  mkdir -p "$CONFIG_DIR"
  local escaped_repo escaped_skill escaped_bin escaped_platform escaped_arch escaped_version escaped_commit escaped_build_date
  escaped_repo="$(json_escape "$SKILL_DIR")"
  escaped_skill="$(json_escape "$AGENTS_DEST")"
  escaped_bin="$(json_escape "$BIN_DIR")"
  escaped_platform="$(json_escape "$(uname -s | tr '[:upper:]' '[:lower:]')")"
  escaped_arch="$(json_escape "$(uname -m)")"
  escaped_version="$(json_escape "$BUILD_VERSION")"
  escaped_commit="$(json_escape "$BUILD_COMMIT")"
  escaped_build_date="$(json_escape "$BUILD_DATE")"
  cat > "$INSTALL_STATE_PATH" <<EOF
{
  "repoPath": "$escaped_repo",
  "installedSkillPath": "$escaped_skill",
  "binDir": "$escaped_bin",
  "platform": "$escaped_platform",
  "arch": "$escaped_arch",
  "version": "$escaped_version",
  "commit": "$escaped_commit",
  "buildDate": "$escaped_build_date",
  "installOnly": $([[ "$INSTALL_ONLY" == "1" ]] && echo "true" || echo "false")
}
EOF
  green "Install state: $INSTALL_STATE_PATH"
}

verify_install() {
  local dest="$BIN_DIR/$BINARY_NAME"
  [[ -x "$dest" ]] || { red "Missing installed binary: $dest"; exit 1; }
  [[ -f "$AGENTS_DEST/SKILL.md" ]] || { red "Installed skill artifact is missing SKILL.md"; exit 1; }

  local resolved=""
  if resolved="$(command -v "$BINARY_NAME" 2>/dev/null)"; then
    if [[ "$resolved" != "$dest" ]]; then
      red "$BINARY_NAME on PATH resolves to $resolved"
      red "Expected: $dest"
      exit 1
    fi
  else
    yellow "$BIN_DIR is not in PATH yet."
    yellow "Add to ~/.zshrc: export PATH=\"\$HOME/.local/bin:\$PATH\""
  fi

  "$dest" version >/dev/null
  "$dest" auth config-path >/dev/null
  green "Verified binary and skill artifact"
}

print ""
green "=== zendesk-management setup ==="
print ""
if [[ "$INSTALL_ONLY" == "1" ]]; then
  yellow "Running safe reinstall flow (--install-only)"
fi
install_go
compute_ldflags
build_cli
install_binary
install_skill_artifact
refresh_links
write_install_state
verify_install
print ""
green "=== Done ==="
