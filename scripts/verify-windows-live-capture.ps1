param(
    [Parameter(Mandatory = $true)]
    [string]$ProxyAddress,

    [Parameter(Mandatory = $true)]
    [string]$HttpTargetUrl,

    [Parameter(Mandatory = $true)]
    [string]$HttpsTargetUrl,

    [Parameter(Mandatory = $true)]
    [string]$SessionPath,

    [Parameter(Mandatory = $true)]
    [string]$ArchScopeEngineExe,

    [string]$JavaTrustStore = "",

    [string]$JavaTrustStorePassword = "changeit",

    [string]$OutputPath = "t581-windows-live-capture-evidence.json",

    [string]$ArchScopeExe = "",

    [string]$ElectronCommand = "electron.cmd",

    [int]$WaitForStopSeconds = 180
)

$ErrorActionPreference = "Stop"

if ($ProxyAddress -notmatch "^(?<host>[^:]+):(?<port>[0-9]+)$") {
    throw "ProxyAddress must use host:port form."
}
if (-not $HttpTargetUrl.StartsWith("http://")) {
    throw "HttpTargetUrl must use http://."
}
if (-not $HttpsTargetUrl.StartsWith("https://")) {
    throw "HttpsTargetUrl must use https://."
}
if (-not (Test-Path $ArchScopeEngineExe -PathType Leaf)) {
    throw "ArchScopeEngineExe does not exist: $ArchScopeEngineExe"
}

$proxyHost = $Matches["host"]
$proxyPort = $Matches["port"]
$proxyUrl = "http://$ProxyAddress"
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("archscope-t581-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot | Out-Null

$results = [System.Collections.Generic.List[object]]::new()

function Add-Marker {
    param([string]$Url, [string]$Marker)
    $separator = if ($Url.Contains("?")) { "&" } else { "?" }
    return $Url + $separator + "archscope_t581_client=" + $Marker
}

function Add-Result {
    param(
        [string]$Client,
        [string]$Transport,
        [string]$Marker,
        [bool]$Available,
        [bool]$Succeeded,
        [int]$ExitCode,
        [string]$Detail
    )

    $results.Add([ordered]@{
        client = $Client
        transport = $Transport
        marker = $Marker
        available = $Available
        succeeded = $Succeeded
        exitCode = $ExitCode
        detail = $Detail
    })
}

function Invoke-Client {
    param(
        [string]$Client,
        [string]$Transport,
        [string]$Marker,
        [string]$Command,
        [string[]]$Arguments
    )

    $resolved = Get-Command $Command -ErrorAction SilentlyContinue
    if ($null -eq $resolved) {
        Add-Result -Client $Client -Transport $Transport -Marker $Marker -Available $false -Succeeded $false -ExitCode -1 -Detail "$Command not found"
        return
    }

    $output = & $resolved.Source @Arguments 2>&1 | Out-String
    $exitCode = $LASTEXITCODE
    Add-Result -Client $Client -Transport $Transport -Marker $Marker -Available $true -Succeeded ($exitCode -eq 0) -ExitCode $exitCode -Detail $output.Trim()
}

function Invoke-Edge {
    param([string]$Transport, [string]$Marker, [string]$Url)
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
    if (-not $edgePath) {
        Add-Result -Client "browser" -Transport $Transport -Marker $Marker -Available $false -Succeeded $false -ExitCode -1 -Detail "Microsoft Edge not found"
        return
    }
    $output = & $edgePath "--headless=new" "--disable-gpu" "--proxy-server=$proxyUrl" "--dump-dom" $Url 2>&1 | Out-String
    $exitCode = $LASTEXITCODE
    Add-Result -Client "browser" -Transport $Transport -Marker $Marker -Available $true -Succeeded ($exitCode -eq 0) -ExitCode $exitCode -Detail $output.Trim()
}

try {
    $httpUrls = [ordered]@{
        "curl" = Add-Marker -Url $HttpTargetUrl -Marker "curl-http"
        "browser" = Add-Marker -Url $HttpTargetUrl -Marker "browser-http"
        "jvm" = Add-Marker -Url $HttpTargetUrl -Marker "jvm-http"
        "electron" = Add-Marker -Url $HttpTargetUrl -Marker "electron-http"
    }
    $httpsUrls = [ordered]@{
        "curl" = Add-Marker -Url $HttpsTargetUrl -Marker "curl-https"
        "browser" = Add-Marker -Url $HttpsTargetUrl -Marker "browser-https"
        "jvm" = Add-Marker -Url $HttpsTargetUrl -Marker "jvm-https"
        "electron" = Add-Marker -Url $HttpsTargetUrl -Marker "electron-https"
    }

    foreach ($transport in @("http", "https")) {
        $urls = if ($transport -eq "http") { $httpUrls } else { $httpsUrls }
        $marker = "curl-$transport"
        $curlArguments = @("--fail", "--silent", "--show-error", "--http1.1")
        if ($transport -eq "https") {
            # The short-lived ArchScope leaf has no CRL distribution point.
            # Schannel must still validate the temporary CA and every other
            # certificate error; only unavailable revocation data is tolerated.
            $curlArguments += "--ssl-revoke-best-effort"
        }
        $curlArguments += @("--proxy", $proxyUrl, $urls["curl"])
        Invoke-Client -Client "curl" -Transport $transport -Marker $marker -Command "curl.exe" -Arguments $curlArguments
        Invoke-Edge -Transport $transport -Marker "browser-$transport" -Url $urls["browser"]
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
    Invoke-Client -Client "jvm" -Transport "http" -Marker "jvm-http" -Command "java.exe" -Arguments @(
        $javaSource, $httpUrls["jvm"], $proxyHost, $proxyPort
    )
    if ($JavaTrustStore -and (Test-Path $JavaTrustStore -PathType Leaf)) {
        Invoke-Client -Client "jvm" -Transport "https" -Marker "jvm-https" -Command "java.exe" -Arguments @(
            "-Djavax.net.ssl.trustStore=$JavaTrustStore",
            "-Djavax.net.ssl.trustStorePassword=$JavaTrustStorePassword",
            $javaSource, $httpsUrls["jvm"], $proxyHost, $proxyPort
        )
    } else {
        Add-Result -Client "jvm" -Transport "https" -Marker "jvm-https" -Available $false -Succeeded $false -ExitCode -1 -Detail "JavaTrustStore is required for JVM HTTPS acceptance"
    }

    $electronSource = Join-Path $tempRoot "t581-electron.cjs"
    @"
const { app, session } = require("electron");
app.setPath("userData", process.env.ARCHSCOPE_T581_USER_DATA);
app.disableHardwareAcceleration();
app.whenReady().then(async () => {
  const clientSession = session.fromPartition("t581-acceptance", { cache: false });
  await clientSession.setProxy({
    mode: "fixed_servers",
    proxyRules: process.env.ARCHSCOPE_T581_PROXY
  });
  const response = await clientSession.fetch(process.env.ARCHSCOPE_T581_URL);
  if (!response.ok) throw new Error("HTTP " + response.status);
  const body = await response.text();
  console.log(response.status + " " + body.length);
  app.quit();
}).catch((error) => {
  console.error(error);
  app.exit(1);
});
"@ | Set-Content -Path $electronSource -Encoding ASCII
    foreach ($transport in @("http", "https")) {
        $urls = if ($transport -eq "http") { $httpUrls } else { $httpsUrls }
        $savedElectronUrl = $env:ARCHSCOPE_T581_URL
        $savedElectronProxy = $env:ARCHSCOPE_T581_PROXY
        $savedElectronUserData = $env:ARCHSCOPE_T581_USER_DATA
        try {
            # Electron treats additional URL-shaped command-line arguments as
            # launch targets on Windows. Pass probe data through the child
            # environment so the application script always remains the sole
            # positional launch target.
            $env:ARCHSCOPE_T581_URL = $urls["electron"]
            $env:ARCHSCOPE_T581_PROXY = $ProxyAddress
            $env:ARCHSCOPE_T581_USER_DATA = Join-Path $tempRoot "electron-$transport"
            Invoke-Client -Client "electron" -Transport $transport -Marker "electron-$transport" -Command $ElectronCommand -Arguments @(
                $electronSource
            )
        } finally {
            $env:ARCHSCOPE_T581_URL = $savedElectronUrl
            $env:ARCHSCOPE_T581_PROXY = $savedElectronProxy
            $env:ARCHSCOPE_T581_USER_DATA = $savedElectronUserData
        }
    }

    Write-Host "Client probes finished. Stop the ArchScope capture in the UI."
    $manifestPath = Join-Path $SessionPath "manifest.json"
    $deadline = [DateTime]::UtcNow.AddSeconds($WaitForStopSeconds)
    $terminal = $false
    while ([DateTime]::UtcNow -lt $deadline) {
        if (Test-Path $manifestPath -PathType Leaf) {
            $manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json
            if ($manifest.state -in @("finalized", "recoverable", "failed")) {
                $terminal = $true
                break
            }
        }
        Start-Sleep -Seconds 1
    }
    if (-not $terminal) {
        throw "Capture did not reach a terminal state within $WaitForStopSeconds seconds."
    }

    $productEvidencePath = Join-Path $tempRoot "product-evidence.json"
    $engineOutput = & $ArchScopeEngineExe "http-capture" "acceptance-evidence" "--session-path" $SessionPath "--out" $productEvidencePath 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) {
        throw "ArchScope evidence readback failed: $engineOutput"
    }
    $productEvidence = Get-Content $productEvidencePath -Raw | ConvertFrom-Json

    $contradictions = [System.Collections.Generic.List[string]]::new()
    if ($productEvidence.session.state -ne "finalized") {
        $contradictions.Add("capture session state is $($productEvidence.session.state), expected finalized")
    }
    if ([int64]$productEvidence.stats.observed -lt [int64]$productEvidence.stats.persisted) {
        $contradictions.Add("observed counter is smaller than persisted counter")
    }
    if ([int64]$productEvidence.stats.persisted -lt 8) {
        $contradictions.Add("fewer than eight HTTP/HTTPS supported-tier rows were persisted")
    }
    foreach ($result in $results) {
        if (-not $result.available) {
            $contradictions.Add("$($result.client)-$($result.transport): required client unavailable")
            continue
        }
        if (-not $result.succeeded) {
            $contradictions.Add("$($result.client)-$($result.transport): client failed")
            continue
        }
        $matchingRows = @($productEvidence.rows | Where-Object { $_.url -like "*archscope_t581_client=$($result.marker)*" })
        if ($matchingRows.Count -eq 0) {
            $contradictions.Add("$($result.client)-$($result.transport): no captured product row")
            continue
        }
        $validRows = @($matchingRows | Where-Object {
            $_.captureMode -eq "proxy_mitm" -and
            $_.fidelity -eq "decoded_wire" -and
            $_.coverage -eq "confirmed" -and
            $_.processAttribution -eq "confirmed" -and
            $_.requestBodyStorage -eq "omitted" -and
            $_.responseBodyStorage -eq "omitted"
        })
        if ($validRows.Count -eq 0) {
            $contradictions.Add("$($result.client)-$($result.transport): captured row contradicts supported-tier contract")
        }
    }

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
        schemaVersion = 2
        task = "T-581"
        generatedAt = [DateTime]::UtcNow.ToString("o")
        platform = [System.Environment]::OSVersion.VersionString
        proxyAddress = $ProxyAddress
        sessionPath = (Resolve-Path $SessionPath).Path
        clients = $results
        capture = $productEvidence
        contradictions = $contradictions
        package = $packageEvidence
    }
    $evidence | ConvertTo-Json -Depth 12 | Set-Content -Path $OutputPath -Encoding UTF8
    Write-Host "Wrote T-581 product-readback evidence to $OutputPath"

    if ($contradictions.Count -gt 0) {
        throw ("T-581 acceptance contradictions: " + ($contradictions -join "; "))
    }
} finally {
    if (Test-Path $tempRoot) {
        Remove-Item -Path $tempRoot -Recurse -Force
    }
}
