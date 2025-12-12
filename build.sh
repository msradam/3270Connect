#!/bin/bash
# build.sh
# Builds 3270Connect for Windows and Linux (with icon embedding for Windows)

set -euo pipefail

echo "======================================"
echo "     Building 3270Connect binaries"
echo "======================================"
echo ""

# Create output folder
mkdir -p dist

# Build Windows icon resource to avoid a blank exe icon
WIN_ARCH="${WIN_ARCH:-amd64}"
RSRC_BIN="${RSRC_BIN:-rsrc}"
if ! command -v "$RSRC_BIN" >/dev/null 2>&1; then
    GOPATH_BIN="$(go env GOPATH)/bin"
    for cand in "$GOPATH_BIN/rsrc" "$GOPATH_BIN/rsrc.exe"; do
        if [ -x "$cand" ]; then
            RSRC_BIN="$cand"
            break
        fi
    done
fi
if ! command -v "$RSRC_BIN" >/dev/null 2>&1 && [ ! -x "$RSRC_BIN" ]; then
    echo "Installing rsrc for icon embedding..."
    go install github.com/akavel/rsrc@latest
    RSRC_BIN="$(go env GOPATH)/bin/rsrc"
    [ -x "${RSRC_BIN}.exe" ] && RSRC_BIN="${RSRC_BIN}.exe"
fi
if command -v "$RSRC_BIN" >/dev/null 2>&1 || [ -x "$RSRC_BIN" ]; then
    echo "Embedding Windows icon (arch=$WIN_ARCH)..."
    "$RSRC_BIN" -arch "$WIN_ARCH" -ico logo.ico -o resource.syso
else
    echo "WARNING: rsrc not available; Windows binary will not have an icon."
fi

# Build Windows terminal version
echo "Building Windows terminal version..."
CGO_ENABLED=0 GOOS=windows GOARCH="$WIN_ARCH" go build -o dist/3270Connect.exe .
echo "Windows build complete."

# Build Linux version
echo "Building Linux version..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/3270Connect_linux .
echo "Linux build complete."

echo ""
echo "✔ Build complete!"
echo "--------------------------------------"
echo "  dist/3270Connect.exe      ← Windows terminal version"
echo "  dist/3270Connect_linux    ← Linux (amd64, static)"
echo "--------------------------------------"
echo ""
