#!/bin/bash
# build.sh
# Builds terminal and GUI versions of 3270Connect for Windows and Linux

set -e

echo "======================================"
echo "     Building 3270Connect binaries"
echo "======================================"
echo ""

# Create output folder
mkdir -p dist

# Build Linux version
echo "Building Linux version..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/3270Connect_linux .
if [ $? -ne 0 ]; then
    echo "❌ Failed to build Linux version."
    exit 1
fi

# Build Windows terminal version
echo "Building Windows terminal version..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/3270Connect.exe .
if [ $? -ne 0 ]; then
    echo "❌ Failed to build Windows version."
    exit 1
fi

echo ""
echo "✅ Build complete!"
echo "--------------------------------------"
echo "  dist/3270Connect.exe      → Windows terminal version"
echo "  dist/3270Connect_linux    → Linux (amd64, static)"
echo "--------------------------------------"
echo ""
