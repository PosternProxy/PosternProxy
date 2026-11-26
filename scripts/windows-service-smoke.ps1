param(
    [Parameter(Mandatory)][string]$AssetsDir,
    [Parameter(Mandatory)][string]$Version,
    [switch]$DisposableHost,
    [switch]$LocalMachine
)
$ErrorActionPreference = 'Stop'
if (-not $IsWindows -or (-not $DisposableHost -and -not $LocalMachine)) { throw 'Requires Windows and explicit -DisposableHost or -LocalMachine authorization.' }
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
if (-not ([Security.Principal.WindowsPrincipal]::new($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Requires an elevated administrator terminal.' }
$assets = (Resolve-Path -LiteralPath $AssetsDir).Path
$dataRoot = Join-Path ([Environment]::GetFolderPath('CommonApplicationData')) 'PosternProxy'
if ((Test-Path -LiteralPath $dataRoot) -or (Get-Service posternproxy,posternproxy-agent,posternproxy-caddy -ErrorAction SilentlyContinue)) { throw 'Refusing to replace existing PosternProxy data or services.' }
$controller = Join-Path $assets 'posternproxy.exe'
$agent = Join-Path $assets 'posternproxy-agent.exe'
$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$work = Join-Path $tempRoot ('posternproxy-service-smoke-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $work | Out-Null
$fixture = $null
$base = 'http://127.0.0.1:18181'
$script:session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
$script:headers = @{}

function Wait-Check([scriptblock]$Check, [string]$Description) {
    for ($attempt = 0; $attempt -lt 120; $attempt++) {
        try { if (& $Check) { Write-Host "PASS $Description"; return } } catch { }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out: $Description"
}
function Invoke-API([string]$Path, [string]$Method = 'GET', $Body = $null) {
    $arguments = @{Uri="$base/api$Path"; Method=$Method; WebSession=$script:session; Headers=$script:headers; TimeoutSec=5}
    if ($null -ne $Body) { $arguments.Body = ConvertTo-Json $Body -Depth 10 -Compress; $arguments.ContentType = 'application/json' }
    Invoke-RestMethod @arguments
}
function Login([string]$Email, [string]$Password) {
    $script:headers = @{}
    $result = Invoke-API '/auth/login' 'POST' @{email=$Email; password=$Password}
    $script:headers = @{Authorization="Bearer $($result.access_token)"; 'X-CSRF-Token'=$script:session.Cookies.GetCookies([uri]$base)['csrf_token'].Value}
}
function Install-Controller {
    $settings = Join-Path $work 'controller-settings.json'
    [IO.File]::WriteAllText($settings, '{"POSTERNPROXY_LISTEN":"127.0.0.1:18181"}')
    $output = & $controller install --assets-dir $assets --port 18181 --config-file $settings 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) {
        $diagnostic = ($output -split "`n" | Where-Object { $_ -match 'installation failed' }) -join ' '
        throw "Controller service installation failed: $diagnostic"
    }
    return $output
}
function Check-ServiceIdentity([string]$Name) {
    $service = Get-CimInstance Win32_Service -Filter "Name='$Name'"
    if ($service.StartName -ine "NT SERVICE\$Name" -or $service.State -ne 'Running') { throw "Unexpected service identity/state: $Name" }
}
function Read-ServiceLog([string]$Path) {
    $stream = [IO.FileStream]::new($Path, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::ReadWrite -bor [IO.FileShare]::Delete)
    $reader = [IO.StreamReader]::new($stream)
    try { return $reader.ReadToEnd() } finally { $reader.Dispose() }
}
function Check-Removal([string]$Role) {
    $installed = Join-Path ([Environment]::GetFolderPath('ProgramFiles')) 'PosternProxy'
    $executables = @((Join-Path $installed "$Role.exe"), (Join-Path $installed 'caddy.exe'))
    Wait-Check {
        $services = @(Get-Service $Role,posternproxy-caddy -ErrorAction SilentlyContinue)
        $processes = @(Get-CimInstance Win32_Process -Filter "Name='$Role.exe' OR Name='caddy.exe'" | Where-Object { $_.ExecutablePath -in $executables })
        $services.Count -eq 0 -and $processes.Count -eq 0
    } "$Role services and child processes removed"
}
try {
    $output = Install-Controller
    $email = [regex]::Match($output, '(?m)^Email: (.+)\r?$').Groups[1].Value.Trim()
    $password = [regex]::Match($output, '(?m)^Password: (.+)\r?$').Groups[1].Value.Trim()
    if (-not $email -or -not $password) { throw 'Initial credentials were not returned after readiness.' }
    Login $email $password
    if ((Invoke-API '/version').version -ne $Version) { throw 'Installed version mismatch.' }
    Check-ServiceIdentity 'posternproxy'
    Check-ServiceIdentity 'posternproxy-caddy'
    Write-Host 'PASS controller installation, login, version and virtual service identities'
    if (Test-Path -LiteralPath (Join-Path $dataRoot 'services/posternproxy-bootstrap.dat')) { throw 'Bootstrap credentials were not removed.' }
    $log = Read-ServiceLog (Join-Path $dataRoot 'posternproxy/service.log')
    if ($log.Contains($password)) { throw 'Initial password was logged.' }
    $keyPath = Join-Path $dataRoot 'posternproxy/secret.key'
    $keyHash = (Get-FileHash -LiteralPath $keyPath -Algorithm SHA256).Hash
    $rerun = Install-Controller
    if ($rerun -match '(?m)^Password:' -or $rerun -notmatch 'Existing admin credentials are unchanged') { throw 'Reinstall changed initial-login behavior.' }
    if ((Get-FileHash -LiteralPath $keyPath -Algorithm SHA256).Hash -ne $keyHash) { throw 'Reinstall replaced the encryption identity.' }
    Restart-Service posternproxy
    Wait-Check { (Invoke-API '/version').version -eq $Version } 'controller restart'
    Login $email $password
    Write-Host 'PASS controller install/reinstall, private bootstrap cleanup and stable secret'
    & $controller uninstall
    if ($LASTEXITCODE -ne 0) { throw 'Controller removal failed.' }
    Check-Removal 'posternproxy'
    if (-not (Test-Path -LiteralPath $keyPath)) { throw 'Removal deleted controller data.' }

    $fixtureData = Join-Path $work 'controller'
    New-Item -ItemType Directory -Path $fixtureData | Out-Null
    $fixturePassword = [Guid]::NewGuid().ToString('N') + '-Disposable'
    $environment = @{
        POSTERNPROXY_LISTEN='127.0.0.1:18182'; POSTERNPROXY_DATA_DIR=$fixtureData;
        POSTERNPROXY_DB_PATH=(Join-Path $fixtureData 'test.db'); POSTERNPROXY_CERT_DIR=(Join-Path $fixtureData 'certs');
        POSTERNPROXY_CERT_RUNTIME_DIR=(Join-Path $fixtureData 'runtime'); POSTERNPROXY_CADDY_API='http://127.0.0.1:1';
        POSTERNPROXY_ADMIN_EMAIL='fixture@example.test'; POSTERNPROXY_ADMIN_PASSWORD=$fixturePassword;
        POSTERNPROXY_BEHIND_TLS_PROXY='false'; POSTERNPROXY_CONTROLLER_URL='http://127.0.0.1:18182'
    }
    $fixture = Start-Process -FilePath $controller -PassThru -WindowStyle Hidden -Environment $environment -RedirectStandardOutput (Join-Path $work 'fixture.out') -RedirectStandardError (Join-Path $work 'fixture.err')
    $base = 'http://127.0.0.1:18182'
    $script:session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    $script:headers = @{}
    Wait-Check { (Invoke-API '/version').version -eq $Version } 'isolated enrollment controller'
    Login 'fixture@example.test' $fixturePassword
    $server = Invoke-API '/servers/' 'POST' @{name='Windows smoke agent'; hostname='127.0.0.1'; deployment_type='manual'}
    $enrollmentPath = Join-Path $work 'agent.json'
    [IO.File]::WriteAllText($enrollmentPath, (ConvertTo-Json $server.enrollment -Compress))
    $agentOutput = & $agent install --assets-dir $assets --config-file $enrollmentPath 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) { throw 'Native agent service installation failed.' }
    Check-ServiceIdentity 'posternproxy-agent'
    Check-ServiceIdentity 'posternproxy-caddy'
    $encrypted = [Text.Encoding]::UTF8.GetString([IO.File]::ReadAllBytes((Join-Path $dataRoot 'services/posternproxy-agent.dat')))
    if ($encrypted.Contains($server.enrollment.agent_token)) { throw 'Enrollment was stored in plaintext.' }
    Wait-Check {
        $states = Invoke-API '/configuration'
        $state = $states | Where-Object server_id -eq $server.id
        $online = (Invoke-API "/servers/$($server.id)/status").online
        $online -and $state.status -eq 'applied' -and $state.desired_revision -eq $state.applied_revision
    } 'agent authentication and applied readiness'

    $pemFixture = [IO.File]::ReadAllText((Join-Path $PSScriptRoot '../frontend/e2e/testdata/cert-fixture.ts'))
    $certPath, $privateKeyPath = (Join-Path $work 'cert.pem'), (Join-Path $work 'key.pem')
    [IO.File]::WriteAllText($certPath, [regex]::Match($pemFixture, '(?s)export const certPem = `([^`]+)`').Groups[1].Value)
    [IO.File]::WriteAllText($privateKeyPath, [regex]::Match($pemFixture, '(?s)export const keyPem = `([^`]+)`').Groups[1].Value)
    $certificate = Invoke-RestMethod -Uri "$base/api/certificates/upload" -Method Post -WebSession $script:session -Headers $script:headers -Form @{
        name='Windows certificate access'; domain_names='["cert-upload.e2e.test"]'; cert_file=(Get-Item $certPath); key_file=(Get-Item $privateKeyPath)
    }
    Invoke-API '/dead-hosts/' 'POST' @{server_id=$server.id; domain_names=@('cert-upload.e2e.test'); certificate_id=$certificate.id; status_code=200; custom_body='windows-tls'} | Out-Null
    $checkTLS = {
        $body = & curl.exe --fail --silent --noproxy '*' --cacert $certPath --resolve 'cert-upload.e2e.test:443:127.0.0.1' 'https://cert-upload.e2e.test/'
        $LASTEXITCODE -eq 0 -and $body -eq 'windows-tls'
    }
    Wait-Check $checkTLS 'Caddy service reads protected runtime certificate and serves verified TLS'
    Restart-Service posternproxy-agent
    Wait-Check $checkTLS 'agent service restart'
    & $agent uninstall
    if ($LASTEXITCODE -ne 0) { throw 'Agent removal failed.' }
    Check-Removal 'posternproxy-agent'
    if (-not (Test-Path -LiteralPath (Join-Path $dataRoot 'services/posternproxy-agent.dat'))) { throw 'Removal deleted protected enrollment configuration.' }
    Write-Host 'PASS agent install, encrypted configuration, TLS, restart and data-preserving removal'
} finally {
    if ($fixture -and -not $fixture.HasExited) { Stop-Process -Id $fixture.Id; $fixture.WaitForExit() }
    foreach ($binary in @($agent, $controller)) { & $binary uninstall 2>&1 | Out-Null }
    $resolvedWork = (Resolve-Path -LiteralPath $work).Path
    if (-not $resolvedWork.StartsWith((Join-Path $tempRoot 'posternproxy-service-smoke-'), [StringComparison]::OrdinalIgnoreCase)) { throw 'Unexpected smoke-test cleanup path.' }
    Remove-Item -LiteralPath $resolvedWork -Recurse -Force
}
