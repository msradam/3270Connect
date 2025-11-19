# build.ps1
# Builds terminal and GUI versions of 3270Connect for Windows and Linux

Write-Host "======================================"
Write-Host "     Building 3270Connect binaries"
Write-Host "======================================"
Write-Host ""

# --- Ensure rsrc exists ---
$rsrcCmd = Get-Command rsrc -ErrorAction SilentlyContinue
if ($rsrcCmd) {
    $rsrcPath = $rsrcCmd.Source
} else {
    $rsrcPath = Join-Path $env:USERPROFILE "go\bin\rsrc.exe"
}

if (-not (Test-Path $rsrcPath)) {
    Write-Host "Installing rsrc (for Windows icon embedding)..."
    go install github.com/akavel/rsrc@latest
    $rsrcPath = Join-Path $env:USERPROFILE "go\bin\rsrc.exe"
}

# --- Check rsrc and logo.ico ---
if (-not (Test-Path $rsrcPath)) {
    Write-Error "❌ Could not find rsrc.exe even after install. Ensure Go bin folder is in PATH."
    exit 1
}
if (-not (Test-Path "logo.ico")) {
    Write-Error "❌ logo.ico not found in project directory: $PWD"
    exit 1
}

# --- Embed Windows icon ---
Write-Host "Embedding icon into the Windows binary..."
& $rsrcPath -ico "logo.ico" -o "resource.syso"
if ($LASTEXITCODE -ne 0) {
    Write-Error "❌ Failed to embed icon via rsrc."
    exit 1
}

# --- Create output folder ---
if (-not (Test-Path "dist")) {
    New-Item -ItemType Directory -Path "dist" | Out-Null
}

# --- Build Windows terminal version ---
Write-Host "Building Windows terminal version..."
go build -o "dist/3270Connect.exe" .
if ($LASTEXITCODE -ne 0) {
    Write-Error "❌ Failed to build Windows version."
    exit 1
}

# --- Build Linux version ---
Write-Host "Building Linux version..."
$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
$oldCGO = $env:CGO_ENABLED

$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

go build -o "dist/3270Connect_linux" .
if ($LASTEXITCODE -ne 0) {
    Write-Error "❌ Failed to build Linux version."
    exit 1
}

# --- Restore environment variables ---
if ($oldGOOS) { $env:GOOS = $oldGOOS } else { Remove-Item Env:GOOS -ErrorAction SilentlyContinue }
if ($oldGOARCH) { $env:GOARCH = $oldGOARCH } else { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue }
if ($oldCGO) { $env:CGO_ENABLED = $oldCGO } else { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue }

Write-Host ""
Write-Host "✅ Build complete!"
Write-Host "--------------------------------------"
Write-Host "  • dist/3270Connect.exe      → Windows terminal version"
Write-Host "  • dist/3270Connect_linux    → Linux (amd64, static)"
Write-Host "--------------------------------------"
Write-Host ""
