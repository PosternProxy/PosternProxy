#Requires -Version 7.0
<#
.SYNOPSIS
    PosternProxy Build Script (PowerShell)
    Produces: posternproxy (controller), posternproxy-agent (agent)
    Requires: Go 1.25+, Node 20+ with npm

.PARAMETER GoOS
    Target operating system. Default: linux

.PARAMETER GoArch
    Target architecture. Default: amd64

.EXAMPLE
    .\scripts\build.ps1
    .\scripts\build.ps1 -GoArch arm64
#>
param(
    [string]$GoOS   = $env:GOOS   ?? "linux",
    [string]$GoArch = $env:GOARCH ?? "amd64"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

###############################################################################
# CONFIGURATION
###############################################################################
$ProjectRoot   = (Resolve-Path "$PSScriptRoot\..").Path
$FrontendDir   = Join-Path $ProjectRoot "frontend"
$UIEmbedDir    = Join-Path $ProjectRoot "internal\ui\dist"
$ControllerBin = Join-Path $ProjectRoot "posternproxy"
$AgentBin      = Join-Path $ProjectRoot "posternproxy-agent"

###############################################################################
# HELPERS
###############################################################################
function Info  { param($msg) Write-Host "[BUILD] $msg" -ForegroundColor Green }
function Fatal { param($msg) Write-Host "[ERROR] $msg" -ForegroundColor Red; exit 1 }

###############################################################################
# PRE-FLIGHT
###############################################################################
if (-not (Get-Command go   -ErrorAction SilentlyContinue)) { Fatal "Go is not installed." }
if (-not (Get-Command node -ErrorAction SilentlyContinue)) { Fatal "Node.js is not installed." }
if (-not (Get-Command npm  -ErrorAction SilentlyContinue)) { Fatal "npm is not installed." }

$GoVer   = (go version) -replace ".*go(\d+\.\d+[\.\d]*).*", '$1'
$NodeVer = (node --version) -replace "^v", ""
Info "Go $GoVer, Node $NodeVer"

###############################################################################
# FRONTEND BUILD
###############################################################################
Info "Building frontend..."
Set-Location $FrontendDir

npm ci --silent
if ($LASTEXITCODE -ne 0) { Fatal "npm ci failed." }

npm run build
if ($LASTEXITCODE -ne 0) { Fatal "npm run build failed." }

$DistSize = (Get-ChildItem -Recurse "dist" | Measure-Object -Property Length -Sum).Sum
$DistSizeMB = [math]::Round($DistSize / 1MB, 1)
Info "Frontend built ($($DistSizeMB)MB): $FrontendDir\dist"

# Copy dist into internal/ui/dist so //go:embed can find it.
Info "Copying dist to $UIEmbedDir..."
if (Test-Path $UIEmbedDir) { Remove-Item -Recurse -Force $UIEmbedDir }
Copy-Item -Recurse "dist" $UIEmbedDir

###############################################################################
# VERSION STRING
###############################################################################
Set-Location $ProjectRoot

$Version = "dev"
try {
    $Version = (git describe --tags --always --dirty 2>$null)
    if ($LASTEXITCODE -ne 0 -or -not $Version) { $Version = "dev" }
} catch { $Version = "dev" }

###############################################################################
# GO BUILD — CONTROLLER
###############################################################################
Info "Building controller binary (GOOS=$GoOS GOARCH=$GoArch)..."

$env:CGO_ENABLED = "0"
$env:GOOS        = $GoOS
$env:GOARCH      = $GoArch

go build `
    -trimpath `
    "-ldflags=-s -w -X main.Version=$Version" `
    -o $ControllerBin `
    ./cmd/posternproxy

if ($LASTEXITCODE -ne 0) { Fatal "Controller build failed." }

$ControllerSize = [math]::Round((Get-Item $ControllerBin).Length / 1MB, 1)
Info "Controller: $ControllerBin ($($ControllerSize)MB)"

###############################################################################
# GO BUILD — AGENT
###############################################################################
Info "Building agent binary (GOOS=$GoOS GOARCH=$GoArch)..."

go build `
    -trimpath `
    "-ldflags=-s -w -X main.Version=$Version" `
    -o $AgentBin `
    ./cmd/posternproxy-agent

if ($LASTEXITCODE -ne 0) { Fatal "Agent build failed." }

$AgentSize = [math]::Round((Get-Item $AgentBin).Length / 1MB, 1)
Info "Agent: $AgentBin ($($AgentSize)MB)"

###############################################################################
# DONE
###############################################################################
# Restore environment variables to avoid polluting the shell session.
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
Remove-Item Env:\GOOS        -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH      -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "  Build complete!" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Controller : $ControllerBin"
Write-Host "  Agent      : $AgentBin"
Write-Host "  Target     : $GoOS/$GoArch"
Write-Host "  Version    : $Version"
Write-Host ""
Write-Host "  Deploy with: bash scripts/install.sh" -ForegroundColor DarkGray
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
