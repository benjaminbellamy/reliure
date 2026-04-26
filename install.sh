#!/usr/bin/env sh
# reliure installer — downloads the latest pre-built binary into ~/.local/bin.
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/benjaminbellamy/reliure/main/install.sh | sh

set -eu

REPO="benjaminbellamy/reliure"
BIN="reliure"
INSTALL_DIR="${HOME}/.local/bin"

err() { printf 'error: %s\n' "$*" >&2; exit 1; }
say() { printf '  %s\n' "$*"; }

case "$(uname -s)" in
    Linux) ;;
    *) err "reliure currently only supports Linux." ;;
esac

case "$(uname -m)" in
    x86_64|amd64) ARCH=amd64 ;;
    *) err "unsupported architecture: $(uname -m). Only linux/amd64 is published right now." ;;
esac

command -v curl >/dev/null 2>&1 || err "curl is required."

# Resolve latest release tag
say "Resolving latest release …"
TAG=$(curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
[ -n "$TAG" ] || err "could not resolve latest release."
say "Latest: ${TAG}"

ASSET="${BIN}-${TAG}-linux-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
SUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/SHA256SUMS"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

say "Downloading ${ASSET} …"
curl -sSfL -o "${TMP}/${ASSET}" "$URL"

say "Verifying checksum …"
curl -sSfL -o "${TMP}/SHA256SUMS" "$SUMS_URL" || err "could not download SHA256SUMS."
EXPECTED=$(grep " ${ASSET}\$" "${TMP}/SHA256SUMS" | awk '{print $1}')
[ -n "$EXPECTED" ] || err "no checksum entry for ${ASSET} in SHA256SUMS."
ACTUAL=$(sha256sum "${TMP}/${ASSET}" | awk '{print $1}')
[ "$EXPECTED" = "$ACTUAL" ] || err "checksum mismatch (expected ${EXPECTED}, got ${ACTUAL})."

mkdir -p "$INSTALL_DIR"
install -m 0755 "${TMP}/${ASSET}" "${INSTALL_DIR}/${BIN}"
say "Installed: ${INSTALL_DIR}/${BIN}"

case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *) printf '\n  Add ~/.local/bin to your PATH:\n\n    export PATH="$HOME/.local/bin:$PATH"\n\n' ;;
esac

printf '\n  Run:  %s\n' "${BIN}"
