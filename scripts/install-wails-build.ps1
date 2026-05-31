# Ensures Wails Windows build assets exist (icon.ico, appicon.png, manifest).
# Run from repo root:  .\scripts\install-wails-build.ps1

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$icon = Join-Path $root "build\windows\icon.ico"
if (Test-Path $icon) {
    Write-Host "[OK] build\windows\icon.ico finnes allerede."
    exit 0
}

Write-Host "[INFO] Mangler build-assets. Oppretter fra Wails vanilla-mal..."
$tmp = Join-Path $env:TEMP ("wails-build-" + [guid]::NewGuid().ToString("n").Substring(0, 8))
wails init -n tmpassets -t vanilla -d $tmp | Out-Null
if (-not (Test-Path (Join-Path $tmp "build\windows\icon.ico"))) {
    Write-Error "wails init klarte ikke lage build-mappen."
}
if (Test-Path (Join-Path $root "build")) {
    Remove-Item -Recurse -Force (Join-Path $root "build")
}
Copy-Item -Recurse (Join-Path $tmp "build") (Join-Path $root "build")
Remove-Item -Recurse -Force $tmp
Write-Host "[OK] build\ kopiert til $root"
