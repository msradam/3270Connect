$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$helperScript = Join-Path $scriptRoot 'scripts/update-binaries.ps1'

if (-not (Test-Path -Path $helperScript)) {
    Write-Error "Could not locate helper script at $helperScript"
    exit 1
}

& $helperScript @PSBoundParameters @Args
