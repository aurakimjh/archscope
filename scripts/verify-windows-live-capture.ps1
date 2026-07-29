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

    [Parameter(Mandatory = $true)]
    [int]$WebViewDebugPort,

    [Parameter(Mandatory = $true)]
    [string]$RecoverySessionPath,

    [string]$JavaTrustStore = "",

    [string]$JavaTrustStorePassword = "changeit",

    [string]$OutputPath = "t581-windows-live-capture-evidence.json",

    [string]$ArchScopeExe = "",

    [string]$ElectronCommand = "electron.cmd",

    [int]$LongSessionRequests = 5000,

    [int]$ReentryMinimumRows = 8,

    [int]$WaitForStopSeconds = 180
)

$ErrorActionPreference = "Stop"

$harnessContractPath = Join-Path $PSScriptRoot "t581-live-capture-harness-contract.json"
if (-not (Test-Path $harnessContractPath -PathType Leaf)) {
    throw "Harness contract does not exist: $harnessContractPath"
}
$harnessContract = Get-Content $harnessContractPath -Raw | ConvertFrom-Json
if (
    -not $harnessContract.requiresArchivedArtifact -or
    -not $harnessContract.unsupportedH2 -or
    -not $harnessContract.unsupportedPinning -or
    -not $harnessContract.quicInvisibility -or
    -not $harnessContract.pageReentry -or
    -not $harnessContract.recovery -or
    -not $harnessContract.fixtureTrafficOnly -or
    -not $harnessContract.artifactOmitsLocalPaths
) {
    throw "Harness contract is incomplete or unsafe."
}

if ($ProxyAddress -notmatch "^(?<host>[^:]+):(?<port>[0-9]+)$") {
    throw "ProxyAddress must use host:port form."
}
if (-not $HttpTargetUrl.StartsWith("http://")) {
    throw "HttpTargetUrl must use http://."
}
if (-not $HttpsTargetUrl.StartsWith("https://")) {
    throw "HttpsTargetUrl must use https://."
}
$httpTargetUri = [Uri]$HttpTargetUrl
$httpsTargetUri = [Uri]$HttpsTargetUrl
$loopbackHosts = @("localhost", "127.0.0.1", "::1")
if ($httpTargetUri.Host -notin $loopbackHosts -or $httpsTargetUri.Host -notin $loopbackHosts) {
    throw "Acceptance traffic must use loopback fixture origins only."
}
if (-not (Test-Path $ArchScopeEngineExe -PathType Leaf)) {
    throw "ArchScopeEngineExe does not exist: $ArchScopeEngineExe"
}
if (-not (Test-Path $SessionPath -PathType Container)) {
    throw "SessionPath does not exist: $SessionPath"
}
if ($WebViewDebugPort -lt 1 -or $WebViewDebugPort -gt 65535) {
    throw "WebViewDebugPort must be between 1 and 65535."
}
if (-not (Test-Path $RecoverySessionPath -PathType Container)) {
    throw "RecoverySessionPath does not exist: $RecoverySessionPath"
}
if ((Resolve-Path $RecoverySessionPath).Path -eq (Resolve-Path $SessionPath).Path) {
    throw "RecoverySessionPath must identify a separate crash-recovered session."
}
if ($LongSessionRequests -lt [int]$harnessContract.minLongSessionRequests) {
    throw "LongSessionRequests must be at least $($harnessContract.minLongSessionRequests)."
}
if ($ReentryMinimumRows -lt 1) {
    throw "ReentryMinimumRows must be positive."
}

$proxyHost = $Matches["host"]
$proxyPort = $Matches["port"]
$proxyUrl = "http://$ProxyAddress"
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("archscope-t581-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot | Out-Null

$results = [System.Collections.Generic.List[object]]::new()
$unsupportedProbes = [System.Collections.Generic.List[object]]::new()

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

function Add-UnsupportedResult {
    param(
        [string]$Scenario,
        [bool]$Available,
        [bool]$Succeeded,
        [string]$Detail
    )
    $unsupportedProbes.Add([ordered]@{
        scenario = $Scenario
        available = $Available
        succeeded = $Succeeded
        detail = $Detail
    })
}

function Protect-Artifact {
    param([string]$Path)
    $identity = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
    $security = [System.Security.AccessControl.FileSecurity]::new()
    $security.SetOwner($identity)
    $security.SetAccessRuleProtection($true, $false)
    $rule = [System.Security.AccessControl.FileSystemAccessRule]::new(
        $identity,
        [System.Security.AccessControl.FileSystemRights]::FullControl,
        [System.Security.AccessControl.AccessControlType]::Allow
    )
    $security.AddAccessRule($rule)
    Set-Acl -Path $Path -AclObject $security
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

function Get-ProductEvidence {
    param([string]$Path, [string]$Label)
    $evidencePath = Join-Path $tempRoot "$Label-product-evidence.json"
    $engineOutput = & $ArchScopeEngineExe "http-capture" "acceptance-evidence" "--session-path" $Path "--out" $evidencePath 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) {
        throw "ArchScope $Label evidence readback failed: $engineOutput"
    }
    return Get-Content $evidencePath -Raw | ConvertFrom-Json
}

function Invoke-CDPEvaluate {
    param([int]$Port, [string]$Expression)
    $targets = @(Invoke-RestMethod -Uri "http://127.0.0.1:$Port/json" -Method Get)
    $target = $targets | Where-Object { $_.type -eq "page" -and $_.webSocketDebuggerUrl } | Select-Object -First 1
    if ($null -eq $target) {
        throw "No WebView2 page target is available on CDP port $Port."
    }
    $socket = [System.Net.WebSockets.ClientWebSocket]::new()
    $token = [System.Threading.CancellationToken]::None
    try {
        $socket.ConnectAsync([Uri]$target.webSocketDebuggerUrl, $token).GetAwaiter().GetResult()
        $request = [ordered]@{
            id = 1
            method = "Runtime.evaluate"
            params = [ordered]@{
                expression = $Expression
                awaitPromise = $true
                returnByValue = $true
            }
        } | ConvertTo-Json -Depth 8 -Compress
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($request)
        $socket.SendAsync(
            [ArraySegment[byte]]::new($bytes),
            [System.Net.WebSockets.WebSocketMessageType]::Text,
            $true,
            $token
        ).GetAwaiter().GetResult()
        while ($true) {
            $buffer = [byte[]]::new(65536)
            $stream = [System.IO.MemoryStream]::new()
            do {
                $received = $socket.ReceiveAsync([ArraySegment[byte]]::new($buffer), $token).GetAwaiter().GetResult()
                if ($received.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) {
                    throw "WebView2 CDP socket closed before the evaluation response."
                }
                $stream.Write($buffer, 0, $received.Count)
            } while (-not $received.EndOfMessage)
            $message = [System.Text.Encoding]::UTF8.GetString($stream.ToArray()) | ConvertFrom-Json
            if ($message.id -ne 1) {
                continue
            }
            if ($message.error -or $message.result.exceptionDetails) {
                throw "WebView2 CDP evaluation failed: $($message | ConvertTo-Json -Depth 8 -Compress)"
            }
            return $message.result.result.value
        }
    } finally {
        if ($socket.State -eq [System.Net.WebSockets.WebSocketState]::Open) {
            $socket.CloseAsync(
                [System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure,
                "done",
                $token
            ).GetAwaiter().GetResult()
        }
        $socket.Dispose()
    }
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

    $pinningHandler = [System.Net.Http.HttpClientHandler]::new()
    $pinningHandler.Proxy = [System.Net.WebProxy]::new($proxyUrl)
    $pinningHandler.UseProxy = $true
    $pinningHandler.ServerCertificateCustomValidationCallback = {
        param($message, $certificate, $chain, $errors)
        return $false
    }
    $pinningClient = [System.Net.Http.HttpClient]::new($pinningHandler)
    try {
        try {
            $pinningResponse = $pinningClient.GetAsync($HttpsTargetUrl).GetAwaiter().GetResult()
            $pinningResponse.Dispose()
            Add-UnsupportedResult -Scenario "certificate-pinning" -Available $true -Succeeded $false -Detail "request unexpectedly trusted the intercepted certificate"
        } catch {
            Add-UnsupportedResult -Scenario "certificate-pinning" -Available $true -Succeeded $true -Detail $_.Exception.Message
        }
    } finally {
        $pinningClient.Dispose()
        $pinningHandler.Dispose()
    }

    $quicMarker = "quic-udp"
    $quicClient = [System.Net.Sockets.UdpClient]::new()
    try {
        $quicPayload = [System.Text.Encoding]::UTF8.GetBytes(
            "archscope_t581_client=$quicMarker"
        )
        $sent = $quicClient.Send($quicPayload, $quicPayload.Length, $proxyHost, [int]$proxyPort)
        Add-UnsupportedResult -Scenario "quic" -Available $true -Succeeded ($sent -eq $quicPayload.Length) -Detail "UDP/QUIC marker sent to the TCP-only explicit-proxy endpoint; product readback must remain empty"
    } finally {
        $quicClient.Dispose()
    }

    $h2Source = Join-Path $tempRoot "ArchScopeT581H2Only.java"
    @"
import java.io.*;
import java.net.*;
import javax.net.ssl.*;
public class ArchScopeT581H2Only {
  public static void main(String[] args) throws Exception {
    var originHost = args[0];
    var originPort = Integer.parseInt(args[1]);
    var socket = new Socket(args[2], Integer.parseInt(args[3]));
    var request = "CONNECT " + originHost + ":" + originPort + " HTTP/1.1\r\nHost: " + originHost + ":" + originPort + "\r\n\r\n";
    socket.getOutputStream().write(request.getBytes(java.nio.charset.StandardCharsets.US_ASCII));
    socket.getOutputStream().flush();
    var response = new ByteArrayOutputStream();
    var matched = 0;
    while (matched < 4) {
      var value = socket.getInputStream().read();
      if (value < 0) throw new EOFException("proxy closed before CONNECT response");
      response.write(value);
      var expected = new int[] {13, 10, 13, 10};
      matched = value == expected[matched] ? matched + 1 : (value == 13 ? 1 : 0);
    }
    var head = response.toString(java.nio.charset.StandardCharsets.US_ASCII);
    if (!head.contains(" 200 ")) throw new IOException("CONNECT failed: " + head);
    var ssl = (SSLSocket)SSLSocketFactory.getDefault().createSocket(socket, originHost, originPort, true);
    var parameters = ssl.getSSLParameters();
    parameters.setApplicationProtocols(new String[] {"h2"});
    ssl.setSSLParameters(parameters);
    ssl.startHandshake();
    if (!"h2".equals(ssl.getApplicationProtocol())) throw new IOException("ALPN was " + ssl.getApplicationProtocol());
    ssl.close();
  }
}
"@ | Set-Content -Path $h2Source -Encoding ASCII
    $java = Get-Command "java.exe" -ErrorAction SilentlyContinue
    if ($null -eq $java -or -not $JavaTrustStore -or -not (Test-Path $JavaTrustStore -PathType Leaf)) {
        Add-UnsupportedResult -Scenario "h2-only" -Available $false -Succeeded $false -Detail "java.exe and JavaTrustStore are required for the h2-only probe"
    } else {
        $httpsUri = [Uri]$HttpsTargetUrl
        $httpsPort = if ($httpsUri.IsDefaultPort) { 443 } else { $httpsUri.Port }
        $h2Output = & $java.Source @(
            "-Djavax.net.ssl.trustStore=$JavaTrustStore",
            "-Djavax.net.ssl.trustStorePassword=$JavaTrustStorePassword",
            $h2Source,
            $httpsUri.Host,
            [string]$httpsPort,
            $proxyHost,
            $proxyPort
        ) 2>&1 | Out-String
        Add-UnsupportedResult -Scenario "h2-only" -Available $true -Succeeded ($LASTEXITCODE -eq 0) -Detail $h2Output.Trim()
    }

    $longHandler = [System.Net.Http.HttpClientHandler]::new()
    $longHandler.Proxy = [System.Net.WebProxy]::new($proxyUrl)
    $longHandler.UseProxy = $true
    $longClient = [System.Net.Http.HttpClient]::new($longHandler)
    $longSucceeded = 0
    try {
        for ($index = 0; $index -lt $LongSessionRequests; $index++) {
            $longUrl = (Add-Marker -Url $HttpTargetUrl -Marker "long-$index") + "&token=t581-secret"
            $longResponse = $longClient.GetAsync($longUrl).GetAwaiter().GetResult()
            try {
                if (-not $longResponse.IsSuccessStatusCode) {
                    throw "Long-session request $index returned HTTP $([int]$longResponse.StatusCode)."
                }
                $longSucceeded++
            } finally {
                $longResponse.Dispose()
            }
        }
    } finally {
        $longClient.Dispose()
        $longHandler.Dispose()
    }
    $longSession = [ordered]@{
        requested = $LongSessionRequests
        succeeded = $longSucceeded
    }

    $reentryExpression = @'
(() => new Promise((resolve, reject) => {
  const buttons = Array.from(document.querySelectorAll("nav button.nav-item"));
  const httpButton = buttons.find((button) => button.getAttribute("aria-current") === "page");
  const otherButton = buttons.find((button) => button !== httpButton);
  if (!httpButton || !otherButton) {
    reject(new Error("navigation buttons not found"));
    return;
  }
  otherButton.click();
  setTimeout(() => {
    httpButton.click();
    setTimeout(() => {
      const firstTable = document.querySelector(".app-main tbody");
      const rowCount = firstTable ? firstTable.querySelectorAll("tr").length : 0;
      const restored = httpButton.getAttribute("aria-current") === "page";
      resolve({ rowCount, restored });
    }, 1000);
  }, 500);
}))()
'@
    $reentry = Invoke-CDPEvaluate -Port $WebViewDebugPort -Expression $reentryExpression
    if (-not $reentry.restored -or [int]$reentry.rowCount -lt $ReentryMinimumRows) {
        throw "Page re-entry did not restore at least $ReentryMinimumRows live rows: $($reentry | ConvertTo-Json -Compress)"
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

    $productEvidence = Get-ProductEvidence -Path $SessionPath -Label "main"
    $recoveryEvidence = Get-ProductEvidence -Path $RecoverySessionPath -Label "recovery"

    $contradictions = [System.Collections.Generic.List[string]]::new()
    if ($productEvidence.session.state -ne "finalized") {
        $contradictions.Add("capture session state is $($productEvidence.session.state), expected finalized")
    }
    if ([int]$productEvidence.schemaVersion -ne [int]$harnessContract.productEvidenceSchemaVersion) {
        $contradictions.Add("product evidence schema is $($productEvidence.schemaVersion), expected $($harnessContract.productEvidenceSchemaVersion)")
    }
    if ([int64]$productEvidence.stats.observed -lt [int64]$productEvidence.stats.persisted) {
        $contradictions.Add("observed counter is smaller than persisted counter")
    }
    if ([int64]$productEvidence.stats.persisted -lt 8) {
        $contradictions.Add("fewer than eight HTTP/HTTPS supported-tier rows were persisted")
    }
    if ([int64]$productEvidence.session.totalRows -lt $LongSessionRequests) {
        $contradictions.Add("long-session product readback has fewer than $LongSessionRequests rows")
    }
    if ($longSucceeded -ne $LongSessionRequests) {
        $contradictions.Add("long-session probe completed $longSucceeded of $LongSessionRequests requests")
    }
    if (-not $productEvidence.redaction.known -or -not $productEvidence.redaction.applied -or [int64]$productEvidence.redaction.counts.query_value -lt $LongSessionRequests) {
        $contradictions.Add("capture-time redaction summary does not prove the long-session token substitutions")
    }
    if ([int64]$productEvidence.session.totalRows -lt [int64]$reentry.rowCount) {
        $contradictions.Add("page re-entry restored more rows than the product store contains")
    }
    if (@($productEvidence.rows).Count -gt [int]$harnessContract.maxArtifactRows) {
        $contradictions.Add("product evidence exceeds the contracted artifact row cap")
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
    foreach ($probe in $unsupportedProbes) {
        if (-not $probe.available) {
            $contradictions.Add("$($probe.scenario): required unsupported-tier probe unavailable")
        } elseif (-not $probe.succeeded) {
            $contradictions.Add("$($probe.scenario): unsupported-tier probe failed")
        }
    }
    $h2Rows = @($productEvidence.rows | Where-Object {
        $_.captureMode -eq "proxy_passthrough" -and
        $_.fidelity -eq "unsupported" -and
        $_.error -like "*h2-only ALPN*"
    })
    if ($h2Rows.Count -eq 0) {
        $contradictions.Add("h2-only probe has no proxy_passthrough/unsupported product row")
    }
    $pinningRows = @($productEvidence.rows | Where-Object {
        $_.captureMode -eq "proxy_not_captured" -and
        $_.fidelity -eq "unsupported" -and
        $_.coverage -eq "confirmed" -and
        $_.processAttribution -eq "confirmed" -and
        $_.error -like "*TLS interception failed*"
    })
    if ($pinningRows.Count -eq 0) {
        $contradictions.Add("pinning probe has no attributed proxy_not_captured/unsupported product row")
    }
    $quicRows = @($productEvidence.rows | Where-Object {
        $_.url -like "*archscope_t581_client=$quicMarker*"
    })
    if ($quicRows.Count -gt 0) {
        $contradictions.Add("UDP/QUIC marker unexpectedly appeared as an explicit-proxy HTTP transaction")
    }
    $dishonestUnsupportedRows = @($productEvidence.rows | Where-Object {
        ($_.captureMode -eq "proxy_passthrough" -or $_.captureMode -eq "proxy_not_captured") -and
        $_.fidelity -eq "semantic"
    })
    if ($dishonestUnsupportedRows.Count -gt 0) {
        $contradictions.Add("unsupported-tier product rows claim semantic fidelity")
    }
    if (
        $recoveryEvidence.session.state -ne "recoverable" -or
        [int64]$recoveryEvidence.session.totalRows -lt 1 -or
        -not $recoveryEvidence.redaction.known -or
        [int64]$recoveryEvidence.stats.persisted -lt [int64]$recoveryEvidence.session.totalRows
    ) {
        $contradictions.Add("recovery product readback lacks a recoverable row, redaction checkpoint, or persisted counter")
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
        schemaVersion = [int]$harnessContract.schemaVersion
        task = "T-581"
        generatedAt = [DateTime]::UtcNow.ToString("o")
        platform = [System.Environment]::OSVersion.VersionString
        proxyAddress = $ProxyAddress
        sessionRef = Split-Path -Leaf (Resolve-Path $SessionPath).Path
        privacy = [ordered]@{
            trafficScope = "loopback_fixture_only"
            fixtureTrafficOnly = $true
            localPathsOmitted = $true
            maxArtifactRows = [int]$harnessContract.maxArtifactRows
            reviewBeforeArchive = $true
        }
        clients = $results
        unsupportedProbes = $unsupportedProbes
        longSession = $longSession
        pageReentry = [ordered]@{
            restored = [bool]$reentry.restored
            restoredRows = [int]$reentry.rowCount
            productRows = [int64]$productEvidence.session.totalRows
            productReadback = $true
        }
        capture = $productEvidence
        recovery = $recoveryEvidence
        contradictions = $contradictions
        package = $packageEvidence
    }
    $evidence | ConvertTo-Json -Depth 12 | Set-Content -Path $OutputPath -Encoding UTF8
    Protect-Artifact -Path $OutputPath
    $artifactHash = (Get-FileHash -Algorithm SHA256 -Path $OutputPath).Hash.ToLowerInvariant()
    "$artifactHash  $([System.IO.Path]::GetFileName($OutputPath))" |
        Set-Content -Path ($OutputPath + ".sha256") -Encoding ASCII
    Protect-Artifact -Path ($OutputPath + ".sha256")
    Write-Host "Wrote T-581 product-readback evidence to $OutputPath"
    Write-Host "SHA256 $artifactHash"

    if ($contradictions.Count -gt 0) {
        throw ("T-581 acceptance contradictions: " + ($contradictions -join "; "))
    }
} finally {
    if (Test-Path $tempRoot) {
        Remove-Item -Path $tempRoot -Recurse -Force
    }
}
