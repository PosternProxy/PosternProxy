function Assert-SmokePortsAvailable([int[]]$TCPPorts = @(), [int[]]$UDPPorts = @()) {
    foreach ($port in $TCPPorts) {
        $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Any, $port)
        try {
            $listener.Server.ExclusiveAddressUse = $true
            $listener.Start()
        } catch { throw "TCP listen port $port is unavailable: $($_.Exception.Message)" }
        finally { $listener.Stop() }
    }
    foreach ($port in $UDPPorts) {
        $socket = [Net.Sockets.UdpClient]::new()
        try {
            $socket.ExclusiveAddressUse = $true
            $socket.Client.Bind([Net.IPEndPoint]::new([Net.IPAddress]::Any, $port))
        } catch { throw "UDP listen port $port is unavailable: $($_.Exception.Message)" }
        finally { $socket.Dispose() }
    }
}

function Protect-SmokeDiagnostic([string]$Text, [string[]]$Secrets = @()) {
    foreach ($secret in $Secrets) {
        if ($secret) { $Text = $Text.Replace($secret, '[REDACTED]') }
    }
    $Text = [regex]::Replace($Text, '(?s)-----BEGIN [^-]+-----.*?-----END [^-]+-----', '[REDACTED PEM]')
    $Text = [regex]::Replace($Text, '(?i)Bearer\s+[^\s"\x27]+', 'Bearer [REDACTED]')
    $Text = [regex]::Replace($Text, 'pp_tok_[A-Za-z0-9_\-]+', '[REDACTED TOKEN]')
    return $Text
}

function Wait-Check(
    [scriptblock]$Check,
    [string]$Description,
    [scriptblock]$OnTimeout,
    [int]$Attempts = 120,
    [int]$DelayMilliseconds = 500
) {
    $lastError = $null
    for ($attempt = 0; $attempt -lt $Attempts; $attempt++) {
        try {
            if (& $Check) { Write-Host "PASS $Description"; return }
            $lastError = $null
        } catch { $lastError = $_.Exception.Message }
        Start-Sleep -Milliseconds $DelayMilliseconds
    }
    if ($lastError) {
        Write-Host (Protect-SmokeDiagnostic "Last polling error: $lastError" $script:diagnosticSecrets)
    }
    if ($OnTimeout) {
        try { & $OnTimeout } catch { Write-Host 'Additional timeout diagnostics unavailable.' }
    }
    throw "Timed out: $Description"
}
