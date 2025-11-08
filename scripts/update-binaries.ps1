<#
.SYNOPSIS
Downloads the latest x3270/s3270 assets and syncs them into the `binaries/` tree.

.DESCRIPTION
Targets the SourceForge RSS feed for the x3270 project. By default it pulls the most recent
64-bit Windows “no install” ZIP, extracts the embedded EXEs, and copies them into
`binaries/windows`. Pass `-LinuxArchiveUrl` when you have a tarball with the Linux binaries
(x3270, s3270, x3270if) so they can be copied into `binaries/linux`, and use
`-RegenerateBindata` to rerun `go-bindata` once the binaries are in place.
#>

[CmdletBinding()]
param (
    [Parameter()]
    [ValidateNotNullOrEmpty()]
    [string]$SourceForgeRssUrl = 'https://sourceforge.net/projects/x3270/rss?path=/x3270',

    [Parameter()]
    [ValidateNotNullOrEmpty()]
    [string]$WindowsPattern = 'wc3270-.*noinstall-64\.zip',

    [Parameter()]
    [string]$LinuxArchiveUrl,

    [Parameter()]
    [switch]$RegenerateBindata
)

Set-StrictMode -Version Latest

function Write-Status($message) {
    $timestamp = (Get-Date).ToString('HH:mm:ss')
    Write-Host "[$timestamp] $message"
}

function Ensure-Directory($path) {
    if (-not (Test-Path $path)) {
        New-Item -ItemType Directory -Path $path | Out-Null
    }
}

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptRoot '..')
$windowsDir = Join-Path $repoRoot 'binaries/windows'
$linuxDir = Join-Path $repoRoot 'binaries/linux'
$tempRoot = Join-Path $env:TEMP ('3270connect-binaries-{0:yyyyMMddHHmmss}' -f (Get-Date))

Ensure-Directory $windowsDir
Ensure-Directory $linuxDir
Ensure-Directory $tempRoot

try {
    Write-Status 'Querying SourceForge release feed...'
    $feed = Invoke-WebRequest -Uri $SourceForgeRssUrl -UseBasicParsing
    $xml = [xml]$feed.Content

    $candidates = $xml.rss.channel.item | Where-Object {
        $_.link -match $WindowsPattern -and $_.title -match $WindowsPattern
    }

    if (-not $candidates) {
        throw "No matching Windows asset found in SourceForge RSS response."
    }

    $latest = $candidates | Sort-Object {[DateTime]::Parse($_.pubDate)} | Select-Object -Last 1
    $fileName = [IO.Path]::GetFileName($latest.title)
    $downloadUrl = $latest.link
    $zipPath = Join-Path $tempRoot $fileName

    Write-Status "Downloading $fileName (published $($latest.pubDate))..."
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing

    Write-Status 'Extracting Windows ZIP...'
    $extractDir = Join-Path $tempRoot 'windows'
    Ensure-Directory $extractDir
    Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

    $windowsTargets = @{
        'wc3270.exe'  = 'wc3270.exe'
        'ws3270.exe'  = 'ws3270.exe'
        's3270.exe'   = 's3270.exe'
        'x3270if.exe' = 'x3270if.exe'
    }

    foreach ($target in $windowsTargets.GetEnumerator()) {
        $found = Get-ChildItem -Path $extractDir -Filter $target.Key -Recurse -File -ErrorAction SilentlyContinue
        if ($found) {
            Copy-Item -Path $found[0].FullName -Destination (Join-Path $windowsDir $target.Value) -Force
            Write-Status "Copied $($target.Key) → $windowsDir"
        }
        else {
            Write-Status "WARNING: $($target.Key) not found inside $fileName"
        }
    }

    if ($LinuxArchiveUrl) {
        $linuxArchiveName = [IO.Path]::GetFileName($LinuxArchiveUrl)
        $linuxArchivePath = Join-Path $tempRoot $linuxArchiveName

        Write-Status "Downloading Linux archive $linuxArchiveName..."
        Invoke-WebRequest -Uri $LinuxArchiveUrl -OutFile $linuxArchivePath -UseBasicParsing

        $linuxExtract = Join-Path $tempRoot 'linux'
        Ensure-Directory $linuxExtract

        if ($LinuxArchiveUrl -match '\.zip($|\?)') {
            Expand-Archive -Path $linuxArchivePath -DestinationPath $linuxExtract -Force
        }
        elseif ($LinuxArchiveUrl -match '\.(?:tar\.gz|tgz)($|\?)') {
            tar -xzf $linuxArchivePath -C $linuxExtract
        }
        else {
            throw "Linux archive format is not supported; provide .zip or .tar.gz/.tgz."
        }

        $linuxTargets = @('x3270','s3270','x3270if')
        foreach ($name in $linuxTargets) {
            $match = Get-ChildItem -Path $linuxExtract -Recurse -Filter $name -File -ErrorAction SilentlyContinue
            if ($match) {
                Copy-Item -Path $match[0].FullName -Destination (Join-Path $linuxDir $name) -Force
                Write-Status "Copied $name → $linuxDir"
            }
            else {
                Write-Status "WARNING: $name not found in the Linux archive"
            }
        }
    }
    else {
        Write-Status 'Skipping Linux update; pass -LinuxArchiveUrl when you have a tarball with x3270/s3270/x3270if.'
    }

    if ($RegenerateBindata) {
        Write-Status 'Regenerating bindata via go-bindata...'
        Push-Location $repoRoot
        try {
            & go-bindata -o binaries/bindata.go -pkg binaries ./binaries/...
        }
        finally {
            Pop-Location
        }
    }
    else {
        Write-Status 'go-bindata regeneration not requested; run go-bindata manually if needed.'
    }
}
finally {
    if (Test-Path $tempRoot) {
        Remove-Item -Path $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
