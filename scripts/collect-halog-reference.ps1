# Generate a genuine native/full-XML pair through an already installed V7.6.4.
# Never installs the vendor product or alters an existing capture.
param(
    [Parameter(Mandatory=$true)][string]$ReferenceExe,
    [Parameter(Mandatory=$true)][string]$FixtureURL,
    [Parameter(Mandatory=$true)][string]$OutputDirectory
)
$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'The genuine reference application requires Windows.' }
Add-Type -AssemblyName System.Net.Http
$root = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot)) + [IO.Path]::DirectorySeparatorChar
$out = [IO.Path]::GetFullPath($OutputDirectory)
if (($out + [IO.Path]::DirectorySeparatorChar).StartsWith($root, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Keep unsanitized reference files outside the repository.'
}
$exe = Get-Item -LiteralPath $ReferenceExe
if (-not $exe.VersionInfo.FileVersion.StartsWith('7.6.4')) { throw 'Reference EXE must identify as V7.6.4.' }
$registered = (Get-Item 'Registry::HKEY_CLASSES_ROOT\CLSID\{D7A90400-DF06-4CDC-88BA-7E684EF650DD}\LocalServer32').GetValue('')
$match = [regex]::Match($registered, '^\s*"?(.+?\.exe)"?(?:\s|$)', 'IgnoreCase')
if (-not $match.Success -or -not [IO.Path]::GetFullPath($match.Groups[1].Value).Equals($exe.FullName, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Original COM registration does not match the supplied reference EXE.'
}
$base = [Uri]$FixtureURL
if ($base.Scheme -ne 'http' -or $base.UserInfo) { throw 'Use the unauthenticated synthetic HTTP fixture.' }
New-Item -ItemType Directory -Force -Path $out | Out-Null
$native = Join-Path $out 'reference.halog'
$xml = Join-Path $out 'reference-full.xml'
if ((Test-Path -LiteralPath $native) -or (Test-Path -LiteralPath $xml)) { throw 'Choose a fresh private output directory.' }
$handler = New-Object System.Net.Http.HttpClientHandler
$handler.UseProxy = $false
$handler.UseCookies = $false
$handler.AllowAutoRedirect = $false
$client = New-Object System.Net.Http.HttpClient($handler)
$client.Timeout = [TimeSpan]::FromSeconds(15)
$client.MaxResponseContentBufferSize = 1048576
$reference = $null
$started = $false
try {
    $preflight = $client.GetAsync(([Uri]::new($base, '/echo'))).GetAwaiter().GetResult()
    $text = $preflight.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    $preflight.Dispose()
    if (-not $text.Contains('archscope-synthetic-response')) { throw 'Fixture marker is missing; refusing to capture an arbitrary server.' }
    $reference = New-Object -ComObject 'HTTPAnalyzerStd.HTTPAnalyzerStandAlone'
    $reference.Visible = $true
    if ($reference.IsLogging -or [int]$reference.Log.Count -ne 0) { throw 'Use a stopped, empty reference log. Existing records will not be cleared.' }
    if (-not $reference.AttachProcessByID([int]$PID)) { throw 'Original process attachment failed.' }
    $reference.Start()
    $started = $true
    foreach ($path in @('/redirect', '/echo?query=one&query=two', '/gzip')) {
        $target = [Uri]::new($base, $path)
        if ($path.StartsWith('/echo')) {
            $content = New-Object System.Net.Http.StringContent('x=one&x=two&marker=archscope-synthetic-request', [Text.Encoding]::UTF8, 'application/x-www-form-urlencoded')
            $response = $client.PostAsync($target, $content).GetAwaiter().GetResult()
            $content.Dispose()
        } else { $response = $client.GetAsync($target).GetAwaiter().GetResult() }
        $null = $response.Content.ReadAsByteArrayAsync().GetAwaiter().GetResult()
        $response.Dispose()
    }
    $reference.Stop()
    $started = $false
    $count = [int]$reference.Log.Count
    if ($count -lt 1) { throw 'Reference did not capture this fixture; record the reach limitation.' }
    # LogType values verified from the vendor's bundled Delphi declarations.
    $reference.SaveLog($native, [int]6)
    $reference.SaveLog($xml, [int]0)
    $manifest = [ordered]@{
        schemaVersion = 1
        status = 'UNVERIFIED'
        referenceVersion = $exe.VersionInfo.FileVersion
        referenceExeSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $exe.FullName).Hash.ToLowerInvariant()
        nativeSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $native).Hash.ToLowerInvariant()
        fullXMLSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $xml).Hash.ToLowerInvariant()
        recordedEntries = $count
        fixtureOnlyIntent = $true
        requiresSanitizationReview = $true
        codecImplemented = $false
        note = 'Genuine pair from one stopped reference capture. Review both files privately before sharing. Pair generation is not codec interoperability proof.'
    }
    $manifest | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $out 'reference-manifest.json') -Encoding UTF8
} finally {
    if ($started -and $reference) { try { $reference.Stop() } catch {} }
    if ($reference) { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($reference) }
    $client.Dispose()
}
