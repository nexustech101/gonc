#!/usr/bin/env sh
# ═══════════════════════════════════════════════════════════════════════
#  build-all.sh – Cross-compile and export binaries to /export
# ═══════════════════════════════════════════════════════════════════════
set -eu

VERSION="${VERSION:-1.0.0}"
LDFLAGS="-s -w -X gonc/cmd.version=${VERSION}"
OUTDIR="${1:-/export}"

mkdir -p "$OUTDIR"

echo "Building gonc v${VERSION}..."

platforms="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

for platform in $platforms; do
    os="${platform%/*}"
    arch="${platform#*/}"
    output="${OUTDIR}/gonc-${os}-${arch}"
    if [ "$os" = "windows" ]; then
        output="${output}.exe"
    fi

    echo "  → ${os}/${arch}"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
        go build -ldflags "$LDFLAGS" -trimpath -o "$output" .
done

echo ""
echo "Generating SHA256SUMS..."
cd "$OUTDIR" && sha256sum * > SHA256SUMS

echo ""
echo "Build complete:"
ls -lh "$OUTDIR/"
