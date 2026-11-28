# Tests, then builds the static ARM binary for the Kobo into dist\.
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

# Prefer the local toolchain in .tools\ if present.
$local = Join-Path $PSScriptRoot '.tools\go\bin\go.exe'
$go = if (Test-Path $local) { $local } else { 'go' }
$env:GOTOOLCHAIN = 'local'

& $go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$env:GOOS = 'linux'; $env:GOARCH = 'arm'; $env:GOARM = '7'; $env:CGO_ENABLED = '0'
try {
    & $go vet ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    & $go build -trimpath -ldflags '-s -w' -o dist\kobodash ./cmd/kobodash
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Remove-Item Env:GOOS, Env:GOARCH, Env:GOARM, Env:CGO_ENABLED
}
# Boot hook for autostart (see the README).
& $go run ./tools/koboroot -o dist\KoboRoot.tgz
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Get-Item dist\kobodash | Select-Object Name, Length
