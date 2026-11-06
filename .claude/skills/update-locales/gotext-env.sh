# Source this file. It puts a Go that satisfies go.mod and a current gotext on the PATH.
# Nothing outside the temp folder is changed.

GOTEXT_VERSION=v0.43.0 # golang.org/x/text. Raise this if gotext can't parse the Go standard library.

_need="go$(sed -n 's/^go //p' go.mod | tr -d '\r')"
_cur="$(GOTOOLCHAIN=local go env GOVERSION 2>/dev/null | tr -d '\r')"

# If the go on the PATH is older than go.mod needs, use that version's toolchain from the module cache.
if [ "$(printf '%s\n%s\n' "$_need" "$_cur" | sort -V | head -1)" != "$_need" ]; then
  _cache="$(GOTOOLCHAIN=local go env GOMODCACHE 2>/dev/null | tr -d '\r')"
  _os="$(GOTOOLCHAIN=local go env GOOS 2>/dev/null | tr -d '\r')"
  _arch="$(GOTOOLCHAIN=local go env GOARCH 2>/dev/null | tr -d '\r')"
  # Outside a module go.mod's version check doesn't apply, so this can download the toolchain if it's missing.
  (cd "${TEMP:-${TMPDIR:-/tmp}}" && GOFLAGS=-mod=mod go mod download "golang.org/toolchain@v0.0.1-$_need.$_os-$_arch" >/dev/null 2>&1)
  _root="$_cache/golang.org/toolchain@v0.0.1-$_need.$_os-$_arch"
  if command -v cygpath >/dev/null 2>&1; then
    export GOROOT="$(cygpath -w "$_root")"
    export PATH="$(cygpath -u "$_root")/bin:$PATH"
  else
    export GOROOT="$_root"
    export PATH="$_root/bin:$PATH"
  fi
fi
export GOTOOLCHAIN=local
hash -r

# Build gotext once. It runs from an empty folder so this repo's go.mod and vendor don't apply.
_tmp="${TEMP:-${TMPDIR:-/tmp}}"
command -v cygpath >/dev/null 2>&1 && _tmp="$(cygpath -u "$_tmp")"
_bin="$_tmp/plansbot-gotext/$GOTEXT_VERSION"
if [ ! -x "$_bin/gotext" ] && [ ! -x "$_bin/gotext.exe" ]; then
  mkdir -p "$_bin/build"
  _gobin="$_bin"
  command -v cygpath >/dev/null 2>&1 && _gobin="$(cygpath -w "$_bin")"
  (cd "$_bin/build" && GOBIN="$_gobin" GOFLAGS=-mod=mod go install "golang.org/x/text/cmd/gotext@$GOTEXT_VERSION") || echo "gotext build failed" >&2
fi
export PATH="$_bin:$PATH"
hash -r

echo "go: $(go env GOVERSION), gotext: $(command -v gotext)"
unset _need _cur _cache _os _arch _root _tmp _bin _gobin
