param(
    [Parameter(Mandatory)][int]$RootProcessId,
    [Parameter(Mandatory)][string]$ExpectedExecutable,
    [string]$ExpectedStartTime = ''
)
$ErrorActionPreference = 'Stop'
$rootProcess = Get-Process -Id $RootProcessId -ErrorAction Stop
try {
    if ($rootProcess.Path -ne [IO.Path]::GetFullPath($ExpectedExecutable)) { throw 'Owned root executable mismatch' }
    $started = $rootProcess.StartTime.ToUniversalTime().ToString('o')
    if ($ExpectedStartTime -and $started -ne $ExpectedStartTime) { throw 'Owned root start identity changed' }
    $responding = $rootProcess.Responding
    $hasWindow = $rootProcess.MainWindowHandle -ne 0
} finally { $rootProcess.Dispose() }
$snapshot = @(Get-CimInstance Win32_Process)
$rootEntry = $snapshot | Where-Object ProcessId -EQ $RootProcessId
if (-not $rootEntry) { throw 'Owned root absent from snapshot' }
$queue = [Collections.Generic.Queue[object]]::new()
$queue.Enqueue($rootEntry)
$seen = @{}
$members = [Collections.Generic.List[object]]::new()
while ($queue.Count) {
    $entry = $queue.Dequeue()
    if ($seen.ContainsKey([string]$entry.ProcessId)) { continue }
    $seen[[string]$entry.ProcessId] = $true
    $process = Get-Process -Id $entry.ProcessId -ErrorAction SilentlyContinue
    if ($process) {
        try {
            $role = 'other-owned-descendant'
            if ($entry.ProcessId -eq $RootProcessId) { $role = 'app' }
            elseif ($entry.Name -eq 'msedgewebview2.exe') {
                $role = 'browser'
                if ($entry.CommandLine -match '--type=renderer') { $role = 'renderer' }
                elseif ($entry.CommandLine -match '--type=gpu-process') { $role = 'gpu' }
                elseif ($entry.CommandLine -match '--type=utility') { $role = 'utility' }
                elseif ($entry.CommandLine -match '--type=crashpad-handler') { $role = 'crashpad' }
            }
            $members.Add([ordered]@{
                # Private identity is converted to a run-local ordinal by the caller.
                identity = "$($entry.ProcessId)/$($entry.CreationDate.ToUniversalTime().ToString('o'))"
                role = $role; privateBytes = $process.PrivateMemorySize64
                workingSetBytes = $process.WorkingSet64; handles = $process.HandleCount
                threads = $process.Threads.Count
            })
        } catch [System.InvalidOperationException] {
            if ($entry.ProcessId -eq $RootProcessId) { throw }
        } finally { $process.Dispose() }
    }
    foreach ($candidate in $snapshot) {
        if ($candidate.ParentProcessId -eq $entry.ProcessId -and $candidate.CreationDate -ge $entry.CreationDate) { $queue.Enqueue($candidate) }
    }
}
[ordered]@{ rootStartTime=$started; responding=$responding; hasMainWindow=$hasWindow; members=@($members.ToArray()) } | ConvertTo-Json -Depth 5 -Compress
