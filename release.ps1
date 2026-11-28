# Builds a new version, signs it and (with -Publish) uploads it as a GitHub
# release, from where the Kobo picks it up over Wi-Fi.
param([switch]$Publish, [string]$Notes = 'Aggiornamento della dashboard')
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

$local = Join-Path $PSScriptRoot '.tools\go\bin\go.exe'
$go = if (Test-Path $local) { $local } else { 'go' }
$env:GOTOOLCHAIN = 'local'

# Minutes since 2026 are always increasing and short to read.
$version = [int64]((Get-Date).ToUniversalTime() - [datetime]'2026-01-01').TotalMinutes

& $go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$env:GOOS = 'linux'; $env:GOARCH = 'arm'; $env:GOARM = '7'; $env:CGO_ENABLED = '0'
try {
    & $go build -trimpath -ldflags "-s -w -X main.version=$version" -o dist\kobodash ./cmd/kobodash
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Remove-Item Env:GOOS, Env:GOARCH, Env:GOARM, Env:CGO_ENABLED
}

& $go run ./tools/release -version $version
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($Publish) {
    # Tag the commit that was built, which must already be on GitHub.
    gh release create "v$version" --target (git rev-parse HEAD) dist\release\kobodash.zip dist\release\kobodash.zip.sig dist\release\version.txt --title "v$version" --notes $Notes --latest
} else {
    Write-Output "Pronta in dist\release (versione $version). Con -Publish la pubblichi su GitHub."
}
