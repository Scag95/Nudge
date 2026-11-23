#!/usr/bin/env bash
# Compiles and runs packaging/windows/setup/encoding-test.iss, which checks the
# Inno Setup firewall helpers (Utf16Base64 and FirewallCommand) against known
# vectors. Exits 0 when all pass; the test's exit code otherwise.
#
# Needs ISCC.exe (Inno Setup 6), found as in build-windows-setup.sh. Runs on
# Windows (Git Bash). Nothing is written to the repository.
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo"

die() { echo "test-windows-setup: $*" >&2; exit 1; }

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

winpath() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s\n' "$1"; fi
}

iss="packaging/windows/setup/encoding-test.iss"
[[ -f "$iss" ]] || die "missing $iss"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "==> ISCC ($iscc) $iss"
MSYS2_ARG_CONV_EXCL='*' "$iscc" /Qp "/O$(winpath "$tmp")" "$(winpath "$repo/$iss")"
exe="$tmp/encoding-test.exe"
[[ -f "$exe" ]] || die "ISCC did not produce $exe"

log="$tmp/encoding-test.log"
rc=0
MSYS2_ARG_CONV_EXCL='*' "$exe" /VERYSILENT /SUPPRESSMSGBOXES /NORESTART \
  "/LOG=$(winpath "$log")" || rc=$?
grep -E 'PASS:|FAIL:|expected |got  |ALL PASSED|FAILED' "$log" 2>/dev/null || true
if ((rc != 0)); then
  echo "test-windows-setup: FAILED (exit code $rc)" >&2
  exit "$rc"
fi
echo "test-windows-setup: ok"
