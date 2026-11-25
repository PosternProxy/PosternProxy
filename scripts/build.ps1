#Requires -Version 7.0
<#
.SYNOPSIS
    PosternProxy Build Script (PowerShell)
    Produces controller, agent, and a custom Caddy build (L4 + rate-limit
    plugins) for the selected target.
    Requires: Go 1.26+, Node 20.19+ or 22.12+ with npm

.PARAMETER GoOS
    Target operating system. Default: linux

.PARAMETER GoArch
    Target architecture. Default: amd64

.EXAMPLE
    .\scripts\build.ps1
    .\scripts\build.ps1 -GoArch arm64
    .\scripts\build.ps1 -GoOS windows -GoArch amd64
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
$CaddyContract = Get-Content (Join-Path $ProjectRoot 'internal/caddybuild/build.sh') -Raw
$CaddyPins = @{}
foreach ($PinMatch in [regex]::Matches($CaddyContract, '(?m)^(CADDY_VERSION|CADDY_L4_VERSION|CADDY_RATELIMIT_VERSION|XCADDY_VERSION|CADDY_GO_TOOLCHAIN|CADDY_SECURITY_REPLACEMENTS)=([^\r\n]+)')) {
    $CaddyPins[$PinMatch.Groups[1].Value] = $PinMatch.Groups[2].Value
}
if ($CaddyPins.Count -ne 6) { throw 'Invalid Caddy build contract' }
$binPath       = Join-Path "$ProjectRoot" "bin"
$FrontendDir   = Join-Path $ProjectRoot "frontend"
$UIEmbedDir    = Join-Path $ProjectRoot "internal\ui\dist"
$BinarySuffix  = if ($GoOS -eq "windows") { ".exe" } else { "" }
$ControllerBin = Join-Path $binPath   "posternproxy$BinarySuffix"
$AgentBin      = Join-Path $binPath   "posternproxy-agent$BinarySuffix"
$CaddyBin      = Join-Path $binPath   "caddy$BinarySuffix"
$Version = $env:POSTERNPROXY_BUILD_VERSION
if (-not $Version) {
    try {
        $Version = (git -C $ProjectRoot describe --tags --always --dirty 2>$null)
        if ($LASTEXITCODE -ne 0 -or -not $Version) { $Version = "dev" }
    } catch { $Version = "dev" }
}

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

if (-not (Test-Path $ProjectRoot)){
    New-Item -ItemType Directory -Path $ProjectRoot
}

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

###############################################################################
# GO BUILD — CONTROLLER
###############################################################################
Info "Building controller binary (GOOS=$GoOS GOARCH=$GoArch)..."

# Detect the real host target so host tools (xcaddy) can be built to run on this
# machine. Use GOHOSTOS/GOHOSTARCH, not GOOS/GOARCH: the latter can be left set
# in the shell from a previous (possibly aborted) run and would wrongly report
# the cross target as the host.
$HostGoOS   = (go env GOHOSTOS)
$HostGoArch = (go env GOHOSTARCH)

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
# CADDY BUILD — custom build with L4 + rate-limit plugins
###############################################################################
# Building Caddy here (on the build host) keeps the Go toolchain off the
# production server. The install subcommand uses the release artifact or local
# --assets-dir instead of compiling on the server. xcaddy honours GOOS/GOARCH
# /CGO_ENABLED env vars set above, so the output matches the selected target.
Info "Building custom Caddy (GOOS=$GoOS GOARCH=$GoArch)..."

# xcaddy itself is a host tool, so its filename uses the HOST exe suffix,
# independent of the build target.
$HostExe   = if ($IsWindows) { ".exe" } else { "" }
$GoPath    = (go env GOPATH)
$XcaddyBin = Join-Path (Join-Path $GoPath "bin") "xcaddy$HostExe"

Info "Installing pinned xcaddy..."
# xcaddy must run on THIS machine, so build it for the host — not the cross
# target. With GOOS/GOARCH still set to the cross target, 'go install' would
# cross-compile xcaddy into bin/<os>_<arch>/ and leave nothing runnable here.
$env:GOOS = $HostGoOS; $env:GOARCH = $HostGoArch
go install "github.com/caddyserver/xcaddy/cmd/xcaddy@$($CaddyPins.XCADDY_VERSION)"
$xcaddyInstallFailed = $LASTEXITCODE -ne 0
$env:GOOS = $GoOS; $env:GOARCH = $GoArch
if ($xcaddyInstallFailed) { Fatal "xcaddy install failed." }

# Use the pinned tool installed above.
$XcaddyCmd = $XcaddyBin

$PreviousGoToolchain = $env:GOTOOLCHAIN
$env:GOTOOLCHAIN = $CaddyPins.CADDY_GO_TOOLCHAIN
$CaddySecurityArgs = @(foreach ($Replacement in ($CaddyPins.CADDY_SECURITY_REPLACEMENTS.Trim("'") -split ' ')) { '--replace'; $Replacement })
& $XcaddyCmd build $CaddyPins.CADDY_VERSION `
    --with "github.com/mholt/caddy-l4@$($CaddyPins.CADDY_L4_VERSION)" `
    --with "github.com/mholt/caddy-ratelimit@$($CaddyPins.CADDY_RATELIMIT_VERSION)" `
    --output $CaddyBin @CaddySecurityArgs
$CaddyBuildFailed = $LASTEXITCODE -ne 0
$env:GOTOOLCHAIN = $PreviousGoToolchain
if ($CaddyBuildFailed) { Fatal "Caddy build failed." }

$CaddySize = [math]::Round((Get-Item $CaddyBin).Length / 1MB, 1)
Info "Caddy: $CaddyBin ($($CaddySize)MB)"

Info "Writing offline installation checksums..."
$ChecksumLines = foreach ($BuiltBinary in @($ControllerBin, $AgentBin, $CaddyBin)) {
    $BinaryHash = (Get-FileHash -LiteralPath $BuiltBinary -Algorithm SHA256).Hash.ToLowerInvariant()
    "$BinaryHash  $(Split-Path -Leaf $BuiltBinary)"
}
$ChecksumLines | Set-Content -LiteralPath (Join-Path $binPath 'checksums.txt') -Encoding utf8NoBOM

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
Write-Host "  Caddy      : $CaddyBin"
Write-Host "  Target     : $GoOS/$GoArch"
Write-Host "  Version    : $Version"
Write-Host ""
Write-Host "  Deploy with: sudo ./posternproxy install --assets-dir ." -ForegroundColor DarkGray
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
