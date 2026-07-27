param(
    [Parameter(Mandatory = $true)]
    [string]$ProxyAddress,

    [Parameter(Mandatory = $true)]
    [string]$TargetUrl,

    [string]$OutputPath = "t581-windows-live-capture-evidence.json",

    [string]$ArchScopeExe = ""
)

$ErrorActionPreference = "Stop"

if ($ProxyAddress -notmatch "^(?<host>[^:]+):(?<port>[0-9]+)$") {
    throw "ProxyAddress must use host:port form."
}

$proxyHost = $Matches["host"]
$proxyPort = $Matches["port"]
$proxyUrl = "http://$ProxyAddress"
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("archscope-t581-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot | Out-Null

$results = [System.Collections.Generic.List[object]]::new()

function Add-Result {
    param(
        [string]$Client,
        [bool]$Available,
        [bool]$Succeeded,
        [int]$ExitCode,
        [string]$Detail
    )

    $results.Add([ordered]@{
        client = $Client
        available = $Available
        succeeded = $Succeeded
        exitCode = $ExitCode
        detail = $Detail
    })
}

function Invoke-Client {
    param(
        [string]$Client,
        [string]$Command,
        [string[]]$Arguments
    )

    $resolved = Get-Command $Command -ErrorAction SilentlyContinue
    if ($null -eq $resolved) {
        Add-Result -Client $Client -Available $false -Succeeded $false -ExitCode -1 -Detail "$Command not found"
        return
    }

    $output = & $resolved.Source @Arguments 2>&1 | Out-String
    $exitCode = $LASTEXITCODE
    Add-Result -Client $Client -Available $true -Succeeded ($exitCode -eq 0) -ExitCode $exitCode -Detail $output.Trim()
}

try {
    Invoke-Client -Client "curl" -Command "curl.exe" -Arguments @(
        "--fail",
        "--silent",
        "--show-error",
        "--http1.1",
        "--proxy",
        $proxyUrl,
        $TargetUrl
    )

    $edge = Get-Command "msedge.exe" -ErrorAction SilentlyContinue
    if ($null -eq $edge) {
        $edgeCandidates = @(
            "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe",
            "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe"
        )
        $edgePath = $edgeCandidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
    } else {
        $edgePath = $edge.Source
    }
    if ($edgePath) {
        $edgeOutput = & $edgePath "--headless=new" "--disable-gpu" "--proxy-server=$proxyUrl" "--dump-dom" $TargetUrl 2>&1 | Out-String
        $edgeExit = $LASTEXITCODE
        Add-Result -Client "browser" -Available $true -Succeeded ($edgeExit -eq 0) -ExitCode $edgeExit -Detail $edgeOutput.Trim()
    } else {
        Add-Result -Client "browser" -Available $false -Succeeded $false -ExitCode -1 -Detail "Microsoft Edge not found"
    }

    $javaSource = Join-Path $tempRoot "ArchScopeT581Client.java"
    @"
import java.net.*;
import java.net.http.*;
public class ArchScopeT581Client {
  public static void main(String[] args) throws Exception {
    var proxy = new InetSocketAddress(args[1], Integer.parseInt(args[2]));
    var client = HttpClient.newBuilder()
        .version(HttpClient.Version.HTTP_1_1)
        .proxy(ProxySelector.of(proxy))
        .build();
    var request = HttpRequest.newBuilder(URI.create(args[0])).GET().build();
    var response = client.send(request, HttpResponse.BodyHandlers.ofString());
    if (response.statusCode() >= 400) throw new RuntimeException("HTTP " + response.statusCode());
    System.out.println(response.statusCode() + " " + response.body().length());
  }
}
"@ | Set-Content -Path $javaSource -Encoding ASCII
    Invoke-Client -Client "jvm" -Command "java.exe" -Arguments @(
        $javaSource,
        $TargetUrl,
        $proxyHost,
        $proxyPort
    )

    $electronSource = Join-Path $tempRoot "t581-electron.cjs"
    @"
const { app, net, session } = require("electron");
app.whenReady().then(async () => {
  await session.defaultSession.setProxy({ proxyRules: process.argv[3] });
  const response = await net.fetch(process.argv[2]);
  if (!response.ok) throw new Error("HTTP " + response.status);
  const body = await response.text();
  console.log(response.status + " " + body.length);
  app.quit();
}).catch((error) => {
  console.error(error);
  app.exit(1);
});
"@ | Set-Content -Path $electronSource -Encoding ASCII
    Invoke-Client -Client "electron" -Command "electron.cmd" -Arguments @(
        $electronSource,
        $TargetUrl,
        $proxyUrl
    )

    $packageEvidence = $null
    if ($ArchScopeExe) {
        if (-not (Test-Path $ArchScopeExe -PathType Leaf)) {
            throw "ArchScopeExe does not exist: $ArchScopeExe"
        }
        $signature = Get-AuthenticodeSignature -FilePath $ArchScopeExe
        $version = (Get-Item $ArchScopeExe).VersionInfo
        $packageEvidence = [ordered]@{
            path = (Resolve-Path $ArchScopeExe).Path
            signatureStatus = [string]$signature.Status
            productName = $version.ProductName
            productVersion = $version.ProductVersion
        }
    }

    $evidence = [ordered]@{
        schemaVersion = 1
        task = "T-581"
        generatedAt = [DateTime]::UtcNow.ToString("o")
        platform = [System.Environment]::OSVersion.VersionString
        proxyAddress = $ProxyAddress
        targetUrl = $TargetUrl
        clients = $results
        package = $packageEvidence
        operatorChecks = @(
            "Confirm all successful supported-tier rows show captureMode=proxy_mitm.",
            "Confirm all successful supported-tier rows show fidelity=decoded_wire.",
            "Confirm process attribution is confirmed for persistent endpoints.",
            "Confirm H2-only, QUIC, and pinned traffic is labeled unsupported or absent, never semantic success.",
            "Confirm page re-entry restores the current session and event-skip recovery reloads the live window.",
            "Confirm stopping capture removes the temporary CA and finalized analysis loads on the same page."
        )
    }
    $evidence | ConvertTo-Json -Depth 8 | Set-Content -Path $OutputPath -Encoding UTF8
    Write-Host "Wrote T-581 evidence to $OutputPath"

    $failedAvailable = @($results | Where-Object { $_.available -and -not $_.succeeded })
    if ($failedAvailable.Count -gt 0) {
        exit 1
    }
} finally {
    if (Test-Path $tempRoot) {
        Remove-Item -Path $tempRoot -Recurse -Force
    }
}
