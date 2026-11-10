# Exercise the installed reference reader/writer without starting capture.
# Payloads stay in a fresh private directory; stdout is metadata only.
param(
    [Parameter(Mandatory=$true)][string]$EngineExe,
    [Parameter(Mandatory=$true)][string]$ReferenceExe,
    [Parameter(Mandatory=$true)][string]$SamplePath,
    [Parameter(Mandatory=$true)][string]$WireRoundTripPath,
    [Parameter(Mandatory=$true)][string]$ProjectionPath,
    [Parameter(Mandatory=$true)][string]$OutputDirectory
)
$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'Windows is required.' }
$engine = (Get-Item -LiteralPath $EngineExe).FullName
$referenceFile = Get-Item -LiteralPath $ReferenceExe
if (-not $referenceFile.VersionInfo.FileVersion.StartsWith('7.6.4')) { throw 'Expected HTTP Analyzer V7.6.4.' }
$registry = [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::ClassesRoot, [Microsoft.Win32.RegistryView]::Registry32)
try {
    $serverKey = $registry.OpenSubKey('CLSID\{D7A90400-DF06-4CDC-88BA-7E684EF650DD}\LocalServer32')
    try { $registered = $serverKey.GetValue('') } finally { $serverKey.Dispose() }
} finally { $registry.Dispose() }
$match = [regex]::Match($registered, '^\s*"?(.+?\.exe)"?(?:\s|$)', 'IgnoreCase')
if (-not $match.Success -or (Get-FileHash -LiteralPath $match.Groups[1].Value).Hash -ne (Get-FileHash -LiteralPath $referenceFile.FullName).Hash) { throw 'Reference registration does not match the executable.' }
$source = (Get-Item -LiteralPath $SamplePath).FullName
$wire = (Get-Item -LiteralPath $WireRoundTripPath).FullName
$sourceHash = (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLowerInvariant()
if ($sourceHash -ne (Get-FileHash -LiteralPath $wire -Algorithm SHA256).Hash.ToLowerInvariant()) { throw 'Low-level round trip is not byte-identical.' }
$out = [IO.Path]::GetFullPath($OutputDirectory)
if (Test-Path -LiteralPath $out) { throw 'Choose a fresh private output directory.' }
$repo = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
if ($out.StartsWith($repo + '\', [StringComparison]::OrdinalIgnoreCase) -and -not $out.StartsWith($repo + '\.tmp\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Keep private artifacts outside Git or under .tmp.' }
New-Item -ItemType Directory -Path $out | Out-Null
function Convert-Session([string]$InputFile, [string]$OutputFile, [string]$Format) {
    & $engine http-capture convert --in $InputFile --out $OutputFile --format $Format
    if ($LASTEXITCODE -ne 0) { throw 'Engine conversion failed.' }
}
function Release-Com($Object) {
    if ($null -ne $Object -and [Runtime.InteropServices.Marshal]::IsComObject($Object)) { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($Object) }
}
function Hash-Text([string]$Text) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($Text)))).Replace('-','').ToLowerInvariant() } finally { $sha.Dispose() }
}
function Read-Utf8($Bytes) {
    if ($null -eq $Bytes -or $Bytes -is [DBNull]) { return '' }
    return [Text.Encoding]::UTF8.GetString([byte[]]$Bytes)
}
function Wait-ReferenceLog($Reference, $Rows) {
    # Require actual entry values as well as Count before comparison.
    $deadline=(Get-Date).AddSeconds(30)
    do {
        $ready=$true; $candidate=$Reference.Log
        try {
            if ([int]$candidate.Count -ne $Rows.Count) { $ready=$false }
            elseif ($Rows.Count -gt 0) {
                foreach ($index in @(0,($Rows.Count-1))) {
                    $item=$candidate.Item($index)
                    try {
                        $status=[int]$item.HTTPStatusCode; if ($status -eq -1) {$status=0}
                        if ($item.Method -cne $Rows[$index].method -or $item.URL -cne $Rows[$index].url -or $status -ne [int]$Rows[$index].statusCode) {$ready=$false}
                    } finally {Release-Com $item}
                }
            }
        } finally {Release-Com $candidate}
        if ($ready) {return}
        Start-Sleep -Milliseconds 200
    } while ((Get-Date) -lt $deadline)
    throw 'Reference entries did not finish loading within 30 seconds.'
}
$golden = Join-Path $repo 'apps\engine-native\internal\capture\halog\testdata\reference-v764.halog'
$fixtureSession = Join-Path $out 'synthetic.httpsession'
Convert-Session $golden $fixtureSession 'session'
$fixture = Get-Content -LiteralPath $fixtureSession -Raw -Encoding UTF8 | ConvertFrom-Json
$base = $fixture.transactions[0] | ConvertTo-Json -Depth 40
$rows = @()
foreach ($size in @(14,245,500,72000)) {
    $row = $base | ConvertFrom-Json
    $row.id = "fixture-$size"
    $row.request.bodyPreview = 'x' * $size
    $row.response.bodyPreview = 'y' * $size
    $row.comment = 'synthetic codec verification'
    $rows += $row
}
$failed = $base | ConvertFrom-Json
$failed.id = 'fixture-failed'; $failed.state = 'failed'; $failed.statusCode = 0; $failed.statusText = ''
$rows += $failed
$pending = $base | ConvertFrom-Json
$pending.id = 'fixture-pending'; $pending.state = 'request_sent'; $pending.statusCode = 0; $pending.statusText = ''; $pending.endedAt = ''
$pending.response.bodyPreview = ''; $pending.response.bodyStorage = 'omitted'; $pending.totalMs=0
$rows += $pending
$redirect = $base | ConvertFrom-Json
$redirect.id = 'fixture-redirect'; $redirect.statusCode = 302; $redirect.statusText = 'Found'
$redirect | Add-Member -NotePropertyName redirectURL -NotePropertyValue 'http://127.0.0.1:8080/target' -Force
$rows += $redirect
$fixture.transactions = $rows
$fixture | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath $fixtureSession -Encoding utf8
$synthetic = Join-Path $out 'synthetic.halog'
Convert-Session $fixtureSession $synthetic 'native'
$sampleSession = Join-Path $out 'sample-redacted.httpsession'
$sampleExport = Join-Path $out 'sample-redacted.halog'
Convert-Session $source $sampleSession 'session'
Convert-Session $sampleSession $sampleExport 'native'
$sample = Get-Content -LiteralPath $sampleSession -Raw -Encoding UTF8 | ConvertFrom-Json
$reference = $null
$log = $null
$restore = $null
$checks = [ordered]@{}
try {
    $reference = New-Object -ComObject 'HTTPAnalyzerStd.HTTPAnalyzerStandAlone'
    $log = $reference.Log
    if ($reference.IsLogging) { throw 'Stop reference capture before verification; active capture will not be interrupted.' }
    if ([int]$log.Count -ne 0) {
        $restore = Join-Path $out 'reference-before-verification.halog'
        $reference.SaveLog($restore, 6)
    }
    Release-Com $log; $log = $null
    $reference.LoadLog($wire)
    # V7's COM wrapper caches ILogEntry objects across LoadLog calls. Fresh
    # activation reads the new native list instead of that former snapshot.
    Release-Com $reference
    $reference = New-Object -ComObject 'HTTPAnalyzerStd.HTTPAnalyzerStandAlone'
    Start-Sleep -Milliseconds 500
    $log = $reference.Log
    if ([int]$log.Count -ne $sample.transactions.Count) { throw 'Private wire round-trip count mismatch.' }
    $checks.wireReferenceRows = [int]$log.Count
    Release-Com $log; $log = $null
    $projection = Get-Content -LiteralPath $ProjectionPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $reference.LoadLog($source)
    Release-Com $reference
    $reference = New-Object -ComObject 'HTTPAnalyzerStd.HTTPAnalyzerStandAlone'
    Start-Sleep -Milliseconds 500
    $log = $reference.Log
    if ([int]$log.Count -ne $projection.Count) { throw 'Source projection count mismatch.' }
    $requestBodies = 0; $responseBodies = 0
    for ($i=0; $i -lt $projection.Count; $i++) {
        $expected = $projection[$i]
        $entry = $log.Item($i); $request = $null; $content = $null
        try {
            if ($entry.Method -cne $expected.method -or (Hash-Text $entry.URL) -ne $expected.urlHash -or [int]$entry.HTTPStatusCode -ne $expected.status -or (Hash-Text ([IO.Path]::GetFileName([string]$entry.ProcessName))) -ne $expected.processHash) { throw "Source projection metadata mismatch at row $i." }
            if ([bool]$entry.IsComplete -ne ($expected.state -in @('complete','failed','aborted'))) {throw "Source completion mismatch at row $i."}
            if ([math]::Abs([double]$entry.TimeDuration - [double]$expected.duration) -gt 1) { throw "Source projection timing mismatch at row $i." }
            $request = $entry.Request; $content = $entry.Content
            if ($expected.requestStorage -eq 'inline') {
                if ((Hash-Text (Read-Utf8 $request.PostData)) -ne $expected.requestHash) { throw "Source request body mismatch at row $i." }; $requestBodies++
            }
            if ($expected.responseStorage -eq 'inline') {
                if ((Hash-Text (Read-Utf8 $content.Data)) -ne $expected.responseHash) { throw "Source response body mismatch at row $i." }; $responseBodies++
            }
        } finally { Release-Com $content; Release-Com $request; Release-Com $entry }
    }
    $checks.sourceProjection = @{rows=$projection.Count; metadata='PASS'; timing='PASS'; requestBodiesCompared=$requestBodies; responseBodiesCompared=$responseBodies}
    Release-Com $log; $log = $null
    foreach ($case in @(@{name='synthetic'; path=$synthetic; rows=$fixture.transactions}, @{name='redactedSample'; path=$sampleExport; rows=$sample.transactions})) {
        $reference.LoadLog($case.path)
        Release-Com $reference
        $reference = New-Object -ComObject 'HTTPAnalyzerStd.HTTPAnalyzerStandAlone'
        Wait-ReferenceLog $reference $case.rows
        $log = $reference.Log
        if ([int]$log.Count -ne $case.rows.Count) { throw "Reference rejected $($case.name)." }
        $completed = 0
        for ($i = 0; $i -lt $case.rows.Count; $i++) {
            $expected = $case.rows[$i]
            $entry = $log.Item($i)
            $request = $null; $content = $null
            try {
                $status = [int]$entry.HTTPStatusCode
                if ($status -eq -1) { $status=0 } # Native uses -1 for no response.
                if ($entry.Method -cne $expected.method -or $entry.URL -cne $expected.url -or $status -ne [int]$expected.statusCode) { throw "Reference metadata mismatch in $($case.name) at row $i." }
                $closed = $expected.state -in @('complete','failed','aborted')
                if ([bool]$entry.IsComplete -ne $closed) { throw "Completion mismatch at row $i." }
                if ($closed -and [math]::Abs([double]$entry.TimeDuration - [math]::Round([double]$expected.totalMs)) -gt 1) { throw "Timing mismatch at row $i." }
                if ($expected.process -and $entry.ProcessName -cne $expected.process.name) { throw "Process label mismatch at row $i." }
                $request = $entry.Request; $content = $entry.Content
                $requestText = Read-Utf8 $request.PostData
                $responseText = Read-Utf8 $content.Data
                if ($requestText -cne [string]$expected.request.bodyPreview -or $responseText -cne [string]$expected.response.bodyPreview) { throw "Retained body mismatch at row $i." }
                if ($closed) { $completed++ }
            } finally { Release-Com $content; Release-Com $request; Release-Com $entry }
        }
        $checks[$case.name] = @{rows=$case.rows.Count; completed=$completed; metadata='PASS'; timing='PASS'; retainedBodies='PASS'; processLabels='PASS'}
        $reference.SaveLog((Join-Path $out ($case.name + '-reference-resaved.halog')), 6)
        Release-Com $log; $log = $null
    }
} finally {
    Release-Com $log
    try { if ($restore) { $reference.LoadLog($restore) } } finally { Release-Com $reference }
}
$reimportRows=@{}
foreach ($name in @('synthetic','redactedSample')) {
    Convert-Session (Join-Path $out ($name + '-reference-resaved.halog')) (Join-Path $out ($name + '-reimport.httpsession')) 'session'
    $reimport = Get-Content -LiteralPath (Join-Path $out ($name+'-reimport.httpsession')) -Raw -Encoding UTF8 | ConvertFrom-Json
    $expectedRows = if ($name -eq 'synthetic') {$fixture.transactions} else {$sample.transactions}
    if ($reimport.transactions.Count -ne $expectedRows.Count) {throw 'Reference reimport count mismatch.'}
    for ($i=0;$i -lt $expectedRows.Count;$i++) {
        $a=$expectedRows[$i];$b=$reimport.transactions[$i]
        if ($a.method -cne $b.method -or $a.url -cne $b.url -or $a.statusCode -ne $b.statusCode -or [math]::Abs($a.totalMs-$b.totalMs) -gt 1 -or [string]$a.request.bodyPreview -cne [string]$b.request.bodyPreview -or [string]$a.response.bodyPreview -cne [string]$b.response.bodyPreview) {throw "Reference reimport mismatch in $name at row $i."}
    }
    $reimportRows[$name]=$reimport.transactions.Count
}
if ($sourceHash -ne (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLowerInvariant()) { throw 'Source changed.' }
$report = [ordered]@{schemaVersion=1; task='T-608'; referenceVersion=$referenceFile.VersionInfo.FileVersion; sourceBytes=(Get-Item -LiteralPath $source).Length; sourceSHA256=$sourceHash; wireByteEquality=$true; sourceUnchanged=$true; referenceReadWrite='PASS'; checks=$checks; referenceReimport=@{status='PASS';rows=$reimportRows;fields='method, URL, status, duration, retained request/response text'}; payloadsPublished=$false}
$json = $report | ConvertTo-Json -Depth 10
$json | Set-Content -LiteralPath (Join-Path $out 'interop.json') -Encoding utf8
$json
