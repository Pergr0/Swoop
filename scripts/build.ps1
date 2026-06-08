<#
.SYNOPSIS
  Swoop build script for Windows.

.DESCRIPTION
  Checks dependencies, installs only what is missing, updates only what is
  outdated (Go/Node by minimum version), builds the app, prints the result and
  the binary location. Can also remove exactly the dependencies it installed.

.PARAMETER Clean
  Remove the dependencies that this script installed (tracked in a state file).

.PARAMETER CheckOnly
  Check and install/update dependencies, but do not build.

.PARAMETER NoPause
  Exit immediately when finished (for CI or nested shells). By default the
  window waits for Enter so you can read errors after a failed build.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts\build.ps1
  powershell -ExecutionPolicy Bypass -File scripts\build.ps1 -CheckOnly
  powershell -ExecutionPolicy Bypass -File scripts\build.ps1 -Clean
  powershell -ExecutionPolicy Bypass -File scripts\build.ps1 -NoPause

  Output is plain ASCII on purpose (no special characters).
#>

[CmdletBinding()]
param(
    [switch]$Clean,
    [switch]$CheckOnly,
    [switch]$NoPause
)

$ErrorActionPreference = "Stop"

$RepoRoot   = Split-Path -Parent $PSScriptRoot
$StateFile  = Join-Path $RepoRoot ".swoop-deps.txt"
$BuildLog   = Join-Path $RepoRoot "build\build.log"
$OutputName = "swoop"

$MinGo   = [version]"1.21"
$MinNode = [version]"18.0"

$GoWingetId   = "GoLang.Go"
$NodeWingetId = "OpenJS.NodeJS.LTS"

function Write-Ok   { param($m) Write-Host "[ OK ] $m" }
function Write-Info { param($m) Write-Host "[INFO] $m" }
function Write-Warn { param($m) Write-Host "[WARN] $m" }
function Write-Fail { param($m) Write-Host "[FAIL] $m" }
function Write-Step { param($m) Write-Host ""; Write-Host "=== $m ===" }

function Wait-ForUser {
    if ($NoPause) { return }
    Write-Host ""
    Read-Host "Press Enter to close"
}

function Format-LogLine {
    param($Object)
    if ($null -eq $Object) { return "" }
    if ($Object -is [System.Management.Automation.ErrorRecord]) {
        return $Object.ToString()
    }
    return $Object.ToString()
}

function Init-BuildLog {
    $dir = Split-Path -Parent $BuildLog
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $header = @(
        "=== Swoop build log ==="
        "started: $(Get-Date -Format o)"
        "host:    $env:COMPUTERNAME"
        "cwd:     $RepoRoot"
        ""
    ) -join "`n"
    Set-Content -Path $BuildLog -Value $header -Encoding UTF8
    Write-Info "build log: $BuildLog"
}

function Write-LogLine {
    param([string]$Line)
    if ([string]::IsNullOrEmpty($Line)) { return }
    Add-Content -Path $BuildLog -Value $Line -Encoding UTF8
}

function Invoke-LoggedCommand {
    param(
        [string]$Label,
        [scriptblock]$Command
    )
    Write-Info $Label
    Write-LogLine ""
    Write-LogLine "=== $Label ==="
    Write-LogLine "$(Get-Date -Format o)"
    $prevEAP = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & $Command *>&1 | ForEach-Object {
            $line = Format-LogLine $_
            if ($line -ne "") {
                Write-Host $line
                Write-LogLine $line
            }
        }
        $code = $LASTEXITCODE
        Write-LogLine "exit code: $code"
        return $code
    } finally {
        $ErrorActionPreference = $prevEAP
    }
}

function Show-BuildLogTail {
    param([int]$Lines = 50)
    if (-not (Test-Path $BuildLog)) { return }
    Write-Step "Build log (last $Lines lines)"
    Write-Info "full log: $BuildLog"
    Get-Content $BuildLog -Tail $Lines -Encoding UTF8 | ForEach-Object { Write-Host $_ }
}

function Show-GoBuildErrors {
    Write-Info "running go build -v for compiler details"
    return (Invoke-LoggedCommand "go build -v ." { go build -v . })
}

function Test-Cmd { param($name) [bool](Get-Command $name -ErrorAction SilentlyContinue) }

function Add-SessionPath {
    # Make freshly installed tools usable without restarting the shell.
    $extra = @(
        "C:\Program Files\Go\bin",
        "C:\Program Files\nodejs",
        (Join-Path $env:USERPROFILE "go\bin")
    )
    foreach ($p in $extra) {
        if (($env:Path -split ";") -notcontains $p) {
            $env:Path = "$p;$env:Path"
        }
    }
}

function Record-Dep {
    param($entry)
    if (-not (Test-Path $StateFile)) { New-Item -ItemType File -Path $StateFile -Force | Out-Null }
    $existing = Get-Content $StateFile -ErrorAction SilentlyContinue
    if ($existing -notcontains $entry) { Add-Content -Path $StateFile -Value $entry }
}

function Get-GoVersion {
    if (-not (Test-Cmd go)) { return $null }
    $out = (& go version) 2>$null
    if ($out -match "go(\d+\.\d+(\.\d+)?)") { return [version]$Matches[1] }
    return $null
}

function Get-NodeVersion {
    if (-not (Test-Cmd node)) { return $null }
    $out = (& node --version) 2>$null
    if ($out -match "v(\d+\.\d+\.\d+)") { return [version]$Matches[1] }
    return $null
}

function Invoke-Winget {
    param([string]$Action, [string]$Id)
    & winget $Action --id $Id -e --accept-source-agreements --accept-package-agreements
    return $LASTEXITCODE
}

function Ensure-WingetTool {
    param([string]$Id, [string]$DisplayName, [version]$Min, [scriptblock]$GetVersion)

    $current = & $GetVersion
    if ($null -ne $current -and $current -ge $Min) {
        Write-Ok "$DisplayName $current present"
        return
    }
    if ($null -ne $current) {
        Write-Info "$DisplayName $current is older than $Min, updating"
        Invoke-Winget -Action "upgrade" -Id $Id | Out-Null
    } else {
        Write-Info "$DisplayName missing, installing"
        $code = Invoke-Winget -Action "install" -Id $Id
        if ($code -eq 0) { Record-Dep "winget:$Id" }
    }
    Add-SessionPath
}

function Ensure-Wails {
    Add-SessionPath
    if (Test-Cmd wails) {
        $v = (& wails version 2>$null | Select-Object -First 1)
        Write-Ok "Wails CLI present ($v)"
        return
    }
    if (-not (Test-Cmd go)) {
        Write-Fail "Go is required to install the Wails CLI"
        throw "Go is required to install the Wails CLI"
    }
    Write-Info "installing Wails CLI via go install"
    & go install github.com/wailsapp/wails/v2/cmd/wails@latest
    if ($LASTEXITCODE -eq 0) { Record-Dep "go:wails" }
    Add-SessionPath
}

function Clean-Deps {
    Write-Step "Cleaning dependencies installed by this script"
    if (-not (Test-Path $StateFile)) {
        Write-Warn "no state file ($StateFile); nothing was installed by this script"
        return
    }
    foreach ($line in Get-Content $StateFile) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        if ($line -like "winget:*") {
            $id = $line.Substring(7)
            Write-Info "winget uninstall $id"
            & winget uninstall --id $id -e --accept-source-agreements 2>$null | Out-Null
        }
        elseif ($line -eq "go:wails") {
            $wailsExe = Join-Path $env:USERPROFILE "go\bin\wails.exe"
            Write-Info "removing wails CLI"
            Remove-Item -Force -ErrorAction SilentlyContinue $wailsExe
        }
        else {
            Write-Warn "unknown state entry: $line"
        }
    }
    Remove-Item -Force $StateFile
    Write-Ok "done; removed dependencies recorded in state file"
}

# Frontend native deps (esbuild) are platform-specific. If node_modules was
# installed on or copied from another OS, wipe it so npm reinstalls cleanly.
function Ensure-FrontendDeps {
    $nm     = Join-Path $RepoRoot "frontend\node_modules"
    $marker = Join-Path $nm ".swoop-platform"
    if (Test-Path $nm) {
        $val = if (Test-Path $marker) { (Get-Content $marker -ErrorAction SilentlyContinue | Select-Object -First 1) } else { "" }
        if ($val -ne "Windows") {
            Write-Warn "frontend\node_modules looks foreign (built on another OS); removing for a clean install"
            Remove-Item -Recurse -Force $nm
        }
    }
}

function Mark-FrontendPlatform {
    $nm = Join-Path $RepoRoot "frontend\node_modules"
    if (Test-Path $nm) { Set-Content -Path (Join-Path $nm ".swoop-platform") -Value "Windows" }
}

function Sync-AppIcon {
    $src = Join-Path $RepoRoot "build\appicon.png"
    $ico = Join-Path $RepoRoot "build\windows\icon.ico"
    $binDir = Join-Path $RepoRoot "build\bin"
    if (-not (Test-Path $src)) {
        Write-Warn "build\appicon.png missing; Wails will use a default W icon"
        return
    }
    # Wails only writes icon.ico when it is missing; regenerate it ourselves and
    # drop stale outputs so Explorer never keeps the default W mark.
    Write-Info "refreshing Windows icon from build\appicon.png"
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $binDir
    Remove-Item -Force -ErrorAction SilentlyContinue $ico
    Get-ChildItem $RepoRoot -Filter "*-res.syso" -ErrorAction SilentlyContinue |
        Remove-Item -Force
    Get-ChildItem (Join-Path $RepoRoot "build\windows") -Filter "rsrc_*.syso" -ErrorAction SilentlyContinue |
        Remove-Item -Force
    Write-Info "regenerating build\windows\icon.ico"
    $geniconDir = Join-Path $PSScriptRoot "genicon"
    $prevRoot = $env:SWOOP_ROOT
    $env:SWOOP_ROOT = $RepoRoot
    Push-Location $geniconDir
    try {
        $code = Invoke-LoggedCommand "go run . (genicon)" { go run . }
        if ($code -ne 0) {
            throw "failed to regenerate icon.ico from build\appicon.png (exit $code)"
        }
    } finally {
        Pop-Location
        if ($null -eq $prevRoot) { Remove-Item Env:SWOOP_ROOT -ErrorAction SilentlyContinue }
        else { $env:SWOOP_ROOT = $prevRoot }
    }
    if (-not (Test-Path $ico)) {
        throw "icon.ico was not created"
    }
}

function Test-FrontendNeedsInstall {
    $fe = Join-Path $RepoRoot "frontend"
    $nm = Join-Path $fe "node_modules"
    $stamp = Join-Path $nm ".swoop-install-stamp"
    if (-not (Test-Path $nm)) { return $true }
    if (-not (Test-Path $stamp)) { return $true }
    $lock = Join-Path $fe "package-lock.json"
    if ((Get-Item $lock).LastWriteTime -gt (Get-Item $stamp).LastWriteTime) { return $true }
    return $false
}

function Test-FrontendNeedsVite {
    $fe = Join-Path $RepoRoot "frontend"
    $dist = Join-Path $fe "dist\index.html"
    if (-not (Test-Path $dist)) { return $true }
    $distTime = (Get-Item $dist).LastWriteTime
    $srcDir = Join-Path $fe "src"
    if (-not (Test-Path $srcDir)) { return $false }
    return [bool](Get-ChildItem $srcDir -Recurse -File | Where-Object { $_.LastWriteTime -gt $distTime })
}

function Build-Frontend {
    $fe = Join-Path $RepoRoot "frontend"
    if (-not (Test-Path (Join-Path $fe "package.json"))) {
        Write-Fail "frontend\package.json missing"
        throw "frontend\package.json missing"
    }
    if (Test-FrontendNeedsInstall) {
        Write-Info "installing frontend dependencies (npm ci)"
        Push-Location $fe
        try {
            $code = Invoke-LoggedCommand "npm ci" { npm ci --no-fund --no-audit }
            if ($code -ne 0) { throw "npm ci failed (exit $code)" }
            Set-Content -Path (Join-Path $fe "node_modules\.swoop-install-stamp") -Value (Get-Date -Format o)
            Mark-FrontendPlatform
        } finally {
            Pop-Location
        }
    }
    if (Test-FrontendNeedsVite) {
        Write-Info "building frontend (vite)"
        Push-Location $fe
        try {
            $code = Invoke-LoggedCommand "npm run build" { npm run build }
            if ($code -ne 0) { throw "npm run build failed (exit $code)" }
        } finally {
            Pop-Location
        }
    } else {
        Write-Info "frontend dist is up to date"
    }
    if (-not (Test-Path (Join-Path $fe "dist\index.html"))) {
        Write-Fail "frontend build did not produce dist\index.html"
        throw "frontend build did not produce dist\index.html"
    }
}

function Invoke-Build {
    Write-Step "Building Swoop"
    Init-BuildLog
    Push-Location $RepoRoot
    try {
        Sync-AppIcon
        Ensure-FrontendDeps
        Build-Frontend
        $code = Invoke-LoggedCommand "wails build -s -f -nocolour" { wails build -s -f -nocolour }
        if ($code -eq 0) {
            Write-Ok "build succeeded"
            Mark-FrontendPlatform
            Show-Binary
            return $true
        }
        Write-Fail "build failed (exit code $code)"
        Show-GoBuildErrors | Out-Null
        Show-BuildLogTail
        return $false
    } finally {
        Pop-Location
    }
}

function Show-Binary {
    $binDir = Join-Path $RepoRoot "build\bin"
    $exe    = Join-Path $binDir "$OutputName.exe"
    Write-Step "Build output"
    if (Test-Path $exe) {
        $item   = Get-Item $exe
        $sizeMB = [math]::Round($item.Length / 1MB, 2)
        Write-Info "binary:   $($item.FullName)"
        Write-Info "size:     $sizeMB MB"
        Write-Info "modified: $($item.LastWriteTime)"
        Write-Info "icon:     embedded from build\appicon.png (check Properties dialog)"
        Write-Info "note:     if Explorer still shows the old Wails W thumbnail, refresh the"
        Write-Info "          shell icon cache: ie4uinit.exe -show  (then reopen the folder)"
    } else {
        Write-Warn "expected binary not found: $exe"
    }
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

$exitCode = 0
try {
    if (-not (Test-Cmd winget)) {
        Write-Warn "winget not found; cannot auto-install Go/Node. Install them manually if missing."
    }

    if ($Clean) {
        Clean-Deps
    } else {
        Write-Step "Checking and installing dependencies (Windows)"
        if (Test-Cmd winget) {
            Ensure-WingetTool -Id $GoWingetId   -DisplayName "Go"   -Min $MinGo   -GetVersion { Get-GoVersion }
            Ensure-WingetTool -Id $NodeWingetId -DisplayName "Node" -Min $MinNode -GetVersion { Get-NodeVersion }
        } else {
            $gv = Get-GoVersion;   if ($gv -and $gv -ge $MinGo)   { Write-Ok "Go $gv present" }   else { Write-Warn "Go $MinGo+ required" }
            $nv = Get-NodeVersion; if ($nv -and $nv -ge $MinNode) { Write-Ok "Node $nv present" } else { Write-Warn "Node $MinNode+ required" }
        }
        Ensure-Wails

        if ($CheckOnly) {
            Write-Ok "dependencies ready (build skipped: -CheckOnly)"
        } else {
            $success = Invoke-Build
            if (-not $success) { $exitCode = 1 }
        }
    }
} catch {
    if ($_.Exception.Message) { Write-Fail $_.Exception.Message }
    else { Write-Fail $_ }
    if (Test-Path $BuildLog) { Show-BuildLogTail }
    $exitCode = 1
} finally {
    Wait-ForUser
}
exit $exitCode
