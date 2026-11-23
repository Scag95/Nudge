#!/usr/bin/env bash
# Build the Windows setup (Inno Setup 6) from the staged release files.
#
# Usage: scripts/build-windows-setup.sh [staging_dir]
#
#   staging_dir  folder with the zip contents (nudge.exe, nudge-background.exe,
#                nudge.ico, toggle-autostart.ps1, firewall.ps1,
#                README-windows.txt). Without it, dist/nudge-<version>-windows-x64.zip
#                (made by scripts/build-windows.sh) is extracted to a temp dir.
#
# Output (nothing else is written to the repository):
#   dist/nudge-setup-<version>-windows-x64.exe
#   dist/nudge-setup-<version>-windows-x64.exe.sha256
#
# Needs ISCC.exe (Inno Setup 6): on PATH, in "C:\Program Files (x86)\Inno
# Setup 6" or in "%LOCALAPPDATA%\Programs\Inno Setup 6"
# (winget install JRSoftware.InnoSetup). Runs on Windows (Git Bash); unzip
# and sha256sum come with Git for Windows. The installer script is
# packaging/windows/nudge.iss.
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo"

die() { echo "build-windows-setup: $*" >&2; exit 1; }

# Version: same detection as scripts/build-windows.sh.
detect_version() {
  local v
  v="$(grep -hoE '^[[:space:]]*(const|var)?[[:space:]]*[vV]ersion[[:space:]]*(string)?[[:space:]]*=[[:space:]]*"[0-9][^"]*"' cmd/server/*.go 2>/dev/null \
    | head -n1 | sed -E 's/.*"([^"]+)".*/\1/' || true)"
  printf '%s\n' "${v:-0.1.0}"
}
version="$(detect_version)"
if [[ ! "$version" =~ ^[0-9A-Za-z][0-9A-Za-z.+_-]*$ ]]; then
  echo "build-windows-setup: invalid version '$version'" >&2
  exit 2
fi

# Numeric x.y.z.w for the setup's version resource (as build-windows.sh).
version4() {
  local num parts
  num="$(printf '%s' "$1" | grep -oE '^[0-9]+(\.[0-9]+){0,3}' || true)"
  IFS=. read -r -a parts <<< "${num:-0}"
  while ((${#parts[@]} < 4)); do parts+=(0); done
  for p in "${parts[@]}"; do
    ((10#$p <= 65535)) || { echo "build-windows-setup: version part $p does not fit a Windows version" >&2; exit 2; }
  done
  printf '%d.%d.%d.%d\n' "$((10#${parts[0]}))" "$((10#${parts[1]}))" "$((10#${parts[2]}))" "$((10#${parts[3]}))"
}
version4="$(version4 "$version")"

# ISCC.exe
find_iscc() {
  local c
  for c in ISCC.exe iscc ISCC; do
    if command -v "$c" >/dev/null 2>&1; then command -v "$c"; return 0; fi
  done
  for c in "/c/Program Files (x86)/Inno Setup 6/ISCC.exe" \
           "/c/Program Files/Inno Setup 6/ISCC.exe" \
           "${LOCALAPPDATA:-}/Programs/Inno Setup 6/ISCC.exe"; do
    if [[ -f "$c" ]]; then printf '%s\n' "$c"; return 0; fi
  done
  return 1
}
iscc="$(find_iscc)" || die "ISCC.exe (Inno Setup 6) not found. Install it with: winget install JRSoftware.InnoSetup"

for tool in sha256sum; do
  command -v "$tool" >/dev/null 2>&1 || die "missing required tool: $tool"
done

# Windows path for ISCC (Git Bash / MSYS); unchanged elsewhere.
winpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s\n' "$1"; fi
}

tmp=""
cleanup() { if [[ -n "$tmp" ]]; then rm -rf "$tmp"; fi; }
trap cleanup EXIT

if [[ $# -ge 1 ]]; then
  staging="$1"
  [[ -d "$staging" ]] || die "staging dir not found: $staging"
else
  zip="dist/nudge-${version}-windows-x64.zip"
  [[ -f "$zip" ]] || die "missing $zip: run scripts/build-windows.sh first, or pass a staging dir"
  command -v unzip >/dev/null 2>&1 || die "missing required tool: unzip"
  tmp="$(mktemp -d)"
  unzip -q "$zip" -d "$tmp/pkg"
  staging="$tmp/pkg"
fi
staging="$(cd "$staging" && pwd)"

for f in nudge.exe nudge-background.exe nudge.ico toggle-autostart.ps1 firewall.ps1 README-windows.txt; do
  [[ -f "$staging/$f" ]] || die "missing $f in $staging"
done

iss="packaging/windows/nudge.iss"
[[ -f "$iss" ]] || die "missing $iss"
name="nudge-setup-${version}-windows-x64.exe"
mkdir -p dist

echo "==> ISCC ($iscc), version $version ($version4), files from $staging"
# MSYS would rewrite the /D and /O switches as paths: disable that for ISCC.
MSYS2_ARG_CONV_EXCL='*' "$iscc" /Qp \
  "/DAppVersion=$version" \
  "/DAppVersionNum=$version4" \
  "/DSrcDir=$(winpath "$staging")" \
  "/O$(winpath "$repo/dist")" \
  "$(winpath "$repo/$iss")"

[[ -f "dist/$name" ]] || die "ISCC did not produce dist/$name"
(cd dist && sha256sum "$name" > "$name.sha256")

echo "==> dist/$name"
ls -l "dist/$name" "dist/$name.sha256"
cat "dist/$name.sha256"
