<#
.SYNOPSIS
Regenerates the embedded assets for 3270Connect from the existing `binaries/` tree.

.DESCRIPTION
This script simply runs `go-bindata` with the current contents of `binaries/` so you
don’t need to re-download anything. Keep the native executables already in
`binaries/linux` and `binaries/windows`, then run this from the repo root whenever you
update those files.
#>

Set-StrictMode -Version Latest

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path $scriptRoot

$goBindata = Get-Command -Name go-bindata -ErrorAction SilentlyContinue
if (-not $goBindata) {
    Write-Error 'go-bindata is not on the PATH; install it via `go install github.com/go-bindata/go-bindata/...` and retry.'
    exit 1
}

Write-Host '[INFO]' (Get-Date).ToString('HH:mm:ss') 'running go-bindata'
Push-Location $repoRoot
try {
    & $goBindata -o binaries/bindata.go -pkg binaries ./binaries/...
}
finally {
    Pop-Location
}
