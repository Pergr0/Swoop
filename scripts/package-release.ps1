# Package build/bin outputs for GitHub Releases.
# Usage: powershell -File scripts\package-release.ps1 [-Version v1.1.0] [-BinDir path]
param(
    [string]$Version = "",
    [string]$BinDir = ""
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
if (-not $BinDir) { $BinDir = Join-Path $RepoRoot "build\bin" }
$OutDir = Join-Path $RepoRoot "build\release"

if (-not $Version) {
    Push-Location $RepoRoot
    try {
        $tag = git describe --tags --abbrev=0 2>$null
        if ($tag) {
            $commits = (git rev-list "${tag}..HEAD" --count 2>$null)
            if ($commits -and [int]$commits -gt 0) {
                $Version = "${tag}-build.$((git rev-parse --short HEAD).Substring(0,7))"
            } else {
                $Version = $tag
            }
        } else {
            $Version = "build.$((git rev-parse --short HEAD).Substring(0,7))"
        }
    } finally {
        Pop-Location
    }
}
$Version = $Version.TrimStart("v")
$Prefix = "Swoop-v$Version"

if (-not (Test-Path $BinDir)) {
    Write-Error "BinDir not found: $BinDir"
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
Get-ChildItem $OutDir -Filter "$Prefix-*" | Remove-Item -Force
Get-ChildItem $OutDir -Filter "SHA256SUMS*" | Remove-Item -Force

function Stage-Dir([string]$Name) {
    $d = Join-Path $env:TEMP "swoop-release-$Name"
    if (Test-Path $d) { Remove-Item -Recurse -Force $d }
    New-Item -ItemType Directory -Force -Path $d | Out-Null
    return $d
}

# Windows
if (Test-Path (Join-Path $BinDir "swoop.exe")) {
    $stage = Stage-Dir "win"
    Copy-Item (Join-Path $BinDir "swoop.exe") $stage
    $zip = Join-Path $OutDir "$Prefix-windows-amd64.zip"
    if (Test-Path $zip) { Remove-Item $zip }
    Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip
    Write-Host "[ OK ] $zip"
}

# Linux desktop (may coexist with swoop.app in a multi-platform release folder)
$linuxBin = Join-Path $BinDir "swoop"
if ((Test-Path $linuxBin) -and -not (Test-Path $linuxBin -PathType Container)) {
    $stage = Stage-Dir "linux"
    foreach ($f in @("swoop", "swoop.png", "swoop.desktop", "install-desktop-entry.sh")) {
        $src = Join-Path $BinDir $f
        if (Test-Path $src) { Copy-Item $src $stage }
    }
    $tar = Join-Path $OutDir "$Prefix-linux-amd64.tar.gz"
    if (Test-Path $tar) { Remove-Item $tar }
    Push-Location $stage
    tar -czf $tar *
    Pop-Location
    Write-Host "[ OK ] $tar"
}

# macOS (swoop.app)
$macApp = Join-Path $BinDir "swoop.app"
if (Test-Path $macApp) {
    $zip = Join-Path $OutDir "$Prefix-macos-arm64.zip"
    if (Test-Path $zip) { Remove-Item $zip }
    Push-Location $BinDir
    Compress-Archive -Path "swoop.app" -DestinationPath $zip
    Pop-Location
    Write-Host "[ OK ] $zip"
}

# Rendezvous server (Linux)
$rendezvous = Join-Path $BinDir "swoop-rendezvous"
if (Test-Path $rendezvous) {
    $stage = Stage-Dir "rendezvous"
    Copy-Item $rendezvous $stage
    $deployDir = Join-Path $RepoRoot "build\deploy"
    if (Test-Path $deployDir) {
        foreach ($f in @("install.sh", "swoop-rendezvous.service")) {
            $src = Join-Path $deployDir $f
            if (Test-Path $src) { Copy-Item $src $stage }
        }
    } else {
        $scriptsDeploy = Join-Path $RepoRoot "scripts\deploy"
        foreach ($f in @("install.sh", "swoop-rendezvous.service")) {
            $src = Join-Path $scriptsDeploy $f
            if (Test-Path $src) { Copy-Item $src $stage }
        }
    }
    $tar = Join-Path $OutDir "$Prefix-rendezvous-linux-amd64.tar.gz"
    if (Test-Path $tar) { Remove-Item $tar }
    Push-Location $stage
    tar -czf $tar *
    Pop-Location
    Write-Host "[ OK ] $tar"
}

# Checksums
$sums = Join-Path $OutDir "SHA256SUMS"
Get-ChildItem $OutDir -File | Where-Object { $_.Name -like "$Prefix-*" } | ForEach-Object {
    $hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower()
    "$hash  $($_.Name)"
} | Set-Content -Encoding ascii -NoNewline:$false $sums
Write-Host "[ OK ] $sums"

$manifest = @"
Swoop release $Prefix
Built from: $(git -C $RepoRoot rev-parse HEAD 2>$null)
Date (UTC): $((Get-Date).ToUniversalTime().ToString("yyyy-MM-dd HH:mm:ss"))

Artifacts:
  $Prefix-windows-amd64.zip       - Windows x64 desktop app
  $Prefix-linux-amd64.tar.gz       - Linux x64 desktop (run install-desktop-entry.sh)
  $Prefix-macos-arm64.zip          - macOS Apple Silicon (swoop.app)
  $Prefix-rendezvous-linux-amd64.tar.gz - VPS signaling server (sudo ./install.sh)

Verify: sha256sum -c SHA256SUMS
Upload: gh release create v$Version build/release/$Prefix-* --repo Pergr0/Swoop
"@
$manifest | Set-Content -Encoding utf8 (Join-Path $OutDir "RELEASE.txt")
Write-Host "[ OK ] $(Join-Path $OutDir 'RELEASE.txt')"
Write-Host ""
Write-Host "Release bundle ready in: $OutDir"
