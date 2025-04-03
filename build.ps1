# build.ps1
# Builds terminal and GUI versions of 3270Connect for Windows and Linux

Write-Output 'Building Windows terminal version...'
go build -o 3270Connect.exe go3270Connect.go

Write-Output 'Building Windows GUI version...'
go build -ldflags="-H=windowsgui" -o 3270Connect_GUI.exe go3270Connect.go

Write-Output 'Building Linux version...'
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o 3270Connect_linux go3270Connect.go

# Reset environment variables to avoid affecting future builds
Remove-Item Env:GOOS
Remove-Item Env:GOARCH

Write-Output ''
Write-Output ' Build complete!'
Write-Output '  - 3270Connect.exe : Windows terminal version'
Write-Output '  - 3270Connect_GUI.exe      : Windows GUI version'
Write-Output '  - 3270Connect_linux        : Linux version'
