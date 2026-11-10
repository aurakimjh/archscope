# Native late-bound ArchScope COM smoke. Register the desktop explicitly first:
# archscope-control.exe --register-com C:\absolute\archscope.exe
param(
    [Parameter(Mandatory=$true)][string]$FixtureURL,
    [Parameter(Mandatory=$true)][string]$OutputDirectory,
    [ValidateSet('explicit_proxy','windows_transparent')][string]$Backend = 'explicit_proxy'
)
$ErrorActionPreference = 'Stop'
# This server intentionally exposes late-bound IDispatch without ITypeInfo.
# PowerShell's COM adapter silently returns null for property reads in that
# case. Invoke the dispatch member explicitly so missing values cannot pass.
function Get-ComProperty($Object, [string]$Name) {
    try {
        $value = $Object.GetType().InvokeMember($Name, [Reflection.BindingFlags]::GetProperty, $null, $Object, $null)
        return ,$value
    } catch {
        throw "COM property '$Name' could not be read: $($_.Exception.Message)"
    }
}
if ($env:OS -ne 'Windows_NT') { throw 'Native Windows COM is required.' }
Add-Type -AssemblyName System.Net.Http
$uri = [Uri]$FixtureURL
if ($uri.Scheme -ne 'http' -or $uri.UserInfo) { throw 'Use an unauthenticated synthetic HTTP fixture.' }
if ($Backend -eq 'windows_transparent' -and ($uri.Port -ne 80 -or $uri.IsLoopback)) {
    throw 'Transparent proof needs a peer IPv4 fixture on port 80.'
}
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$controller = $null
$handler = $null
$client = $null
$started = $false
$report = [ordered]@{
    schemaVersion = 1
    scope = 'ArchScope late-bound COM contract; not vendor binary compatibility or full native UI parity'
    platform = 'win32'
    backend = $Backend
    nativeWindowVerified = $false
    referenceInterop = $false
    result = 'UNVERIFIED'
}
try {
    $controller = New-Object -ComObject 'ArchScope.HTTPAnalyzerStandAlone'
    $report.buildCommit = [string](Get-ComProperty $controller 'BuildCommit')
    $controller.Visible = $true
    $controller.Backend = $Backend
    $controller.AttachProcessByID([int]$PID)
    $controller.Start()
    $started = $true
    $report.sessionId = [string](Get-ComProperty $controller 'SessionID')
    if (-not (Get-ComProperty $controller 'IsLogging')) { throw 'COM Start did not start recording.' }
    $controller.Pause()
    if (-not (Get-ComProperty $controller 'IsPaused')) { throw 'Pause was not visible through COM.' }
    $controller.Resume()
    $handler = New-Object System.Net.Http.HttpClientHandler
    $handler.UseCookies = $false
    $handler.AllowAutoRedirect = $false
    if ($Backend -eq 'explicit_proxy') {
        $handler.Proxy = New-Object System.Net.WebProxy("http://$(Get-ComProperty $controller 'ProxyAddress')", $false)
        $handler.UseProxy = $true
    } else {
        $handler.UseProxy = $false
    }
    $client = New-Object System.Net.Http.HttpClient($handler)
    $client.Timeout = [TimeSpan]::FromSeconds(15)
    $client.MaxResponseContentBufferSize = 1048576
    $response = $client.GetAsync($FixtureURL).GetAwaiter().GetResult()
    $null = $response.Content.ReadAsByteArrayAsync().GetAwaiter().GetResult()
    $response.Dispose()
    $controller.Stop()
    $started = $false
    if (Get-ComProperty $controller 'IsLogging') { throw 'COM Stop left recording active.' }
    $log = Get-ComProperty $controller 'Log'
    $count = [int](Get-ComProperty $log 'Count')
    if ($count -lt 1) { throw 'No persisted COM log entries.' }
    $matched = 0
    foreach ($entry in $log) {
        if ([int](Get-ComProperty $entry 'ProcessID') -eq $PID) { $matched++ }
        [void][System.Runtime.InteropServices.Marshal]::FinalReleaseComObject($entry)
    }
    if ($matched -lt 1) { throw 'Caller PID attribution was not preserved.' }
    $first = $log.Item(0)
    if (-not [string](Get-ComProperty $first 'URL')) { throw 'Indexed log lookup failed.' }
    [void][System.Runtime.InteropServices.Marshal]::FinalReleaseComObject($first)
    [void][System.Runtime.InteropServices.Marshal]::FinalReleaseComObject($log)
    $controller.SaveLog((Join-Path ([IO.Path]::GetFullPath($OutputDirectory)) 'fixture.httpsession'), 'session')
    $report.entries = $count
    $report.attributedEntries = $matched
    $report.result = 'PASS'
} catch {
    $report.result = 'FAIL'
    throw
} finally {
    if ($started -and $controller) { try { $controller.Stop() } catch {} }
    if ($client) { $client.Dispose() } elseif ($handler) { $handler.Dispose() }
    if ($controller) { [void][System.Runtime.InteropServices.Marshal]::FinalReleaseComObject($controller) }
    $report | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $OutputDirectory 'com-result.json') -Encoding UTF8
}
