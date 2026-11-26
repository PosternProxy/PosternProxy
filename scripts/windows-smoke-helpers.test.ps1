$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-smoke-helpers.ps1')
$script:diagnosticSecrets = @('disposable-enrollment-secret', 'disposable-session-secret')

$listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Any, 0)
$listener.Start()
$tcpPort = $listener.LocalEndpoint.Port
try {
    $blocked = $false
    try { Assert-SmokePortsAvailable -TCPPorts @($tcpPort) }
    catch { $blocked = $_.Exception.Message -like "TCP listen port $tcpPort is unavailable:*" }
    if (-not $blocked) { throw 'TCP port preflight accepted an occupied port.' }
} finally { $listener.Stop() }
Assert-SmokePortsAvailable -TCPPorts @($tcpPort)

$socket = [Net.Sockets.UdpClient]::new(0)
$udpPort = $socket.Client.LocalEndPoint.Port
try {
    $blocked = $false
    try { Assert-SmokePortsAvailable -UDPPorts @($udpPort) }
    catch { $blocked = $_.Exception.Message -like "UDP listen port $udpPort is unavailable:*" }
    if (-not $blocked) { throw 'UDP port preflight accepted an occupied port.' }
} finally { $socket.Dispose() }
Assert-SmokePortsAvailable -UDPPorts @($udpPort)
Write-Host 'PASS port preflight rejects occupied TCP/UDP ports and accepts released ports'

$text = 'disposable-enrollment-secret disposable-session-secret Bearer unknown-secret pp_tok_another-secret -----BEGIN PRIVATE KEY-----private-bytes-----END PRIVATE KEY-----'
$redacted = Protect-SmokeDiagnostic $text $script:diagnosticSecrets
foreach ($secret in @('disposable-enrollment-secret', 'disposable-session-secret', 'unknown-secret', 'pp_tok_another-secret', 'private-bytes')) {
    if ($redacted.Contains($secret)) { throw 'Diagnostic redaction leaked a credential.' }
}

$script:attempt = 0
Wait-Check {
    $script:attempt++
    $script:attempt -eq 2
} 'poll retries until ready' -Attempts 2 -DelayMilliseconds 0
if ($script:attempt -ne 2) { throw 'Polling did not retry.' }

$script:diagnosed = $false
$script:failed = $false
$output = & {
    try {
        Wait-Check { throw 'Request failed: Bearer unknown-secret disposable-enrollment-secret' } 'expected timeout' -Attempts 1 -DelayMilliseconds 0 -OnTimeout {
            $script:diagnosed = $true
            Write-Host 'Timeout diagnostics captured'
        }
    } catch {
        $script:failed = $_.Exception.Message -eq 'Timed out: expected timeout'
    }
} 6>&1 | Out-String
if (-not $script:failed -or -not $script:diagnosed) { throw 'Timeout failed to retain failure and gather diagnostics.' }
if ($output -notmatch 'Last polling error:' -or $output -notmatch 'Timeout diagnostics captured') { throw 'Polling swallowed its diagnostic output.' }
if ($output.Contains('unknown-secret') -or $output.Contains('disposable-enrollment-secret')) { throw 'Polling logged a credential.' }
Write-Host 'PASS diagnostic redaction and timeout reporting'
