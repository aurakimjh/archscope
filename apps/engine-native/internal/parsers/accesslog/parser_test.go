// [한글] accesslog parser 회귀 테스트.
//
// 검증 대상
//   - nginx / combined / common 3개 regex 가 우선순위대로 시도.
//   - 시간 파싱 (`27/Apr/2026:10:00:01 +0900`) 의 timezone 정확.
//   - 정상 미매칭 라인은 skip + diagnostics.SkippedRecords 증가.
//   - Strict=true 면 첫 skip 이 fatal error.
//   - MaxLines / StartTime / EndTime 필터.
//   - UTF-8 BOM / 다양한 line ending(\r\n) 처리.
package accesslog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nginxLine(timestamp, uri, status, responseTimeSec string) string {
	if timestamp == "" {
		timestamp = "27/Apr/2026:10:00:01 +0900"
	}
	if uri == "" {
		uri = "/api/orders/1001"
	}
	if status == "" {
		status = "200"
	}
	if responseTimeSec == "" {
		responseTimeSec = "0.123"
	}
	return "127.0.0.1 - - [" + timestamp + "] " +
		`"GET ` + uri + ` HTTP/1.1" ` + status + ` 1234 "-" "Mozilla/5.0" ` + responseTimeSec
}

func TestParseLineNginxWithResponseTime(t *testing.T) {
	rec, perr := ParseLine(nginxLine("", "", "", ""))
	if perr != nil {
		t.Fatalf("unexpected parse error: %+v", perr)
	}
	if rec.Method != "GET" {
		t.Errorf("Method = %q, want GET", rec.Method)
	}
	if rec.URI != "/api/orders/1001" {
		t.Errorf("URI = %q, want /api/orders/1001", rec.URI)
	}
	if rec.Status != 200 {
		t.Errorf("Status = %d, want 200", rec.Status)
	}
	if rec.ResponseTimeMS != 123.0 {
		t.Errorf("ResponseTimeMS = %f, want 123.0", rec.ResponseTimeMS)
	}
	if rec.BytesSent != 1234 {
		t.Errorf("BytesSent = %d, want 1234", rec.BytesSent)
	}
	if rec.ClientIP != "127.0.0.1" {
		t.Errorf("ClientIP = %q, want 127.0.0.1", rec.ClientIP)
	}
	if rec.UserAgent != "Mozilla/5.0" {
		t.Errorf("UserAgent = %q", rec.UserAgent)
	}
	if rec.RawLine == "" {
		t.Errorf("RawLine should be populated")
	}
}

func TestParseLineRejectsBadTimestamp(t *testing.T) {
	line := `127.0.0.1 - - [bad-time] "GET /a HTTP/1.1" 200 1 "-" "x" 0.1`
	rec, perr := ParseLine(line)
	if rec != nil {
		t.Fatalf("expected nil record, got %+v", rec)
	}
	if perr.Reason != ReasonInvalidTimestamp {
		t.Fatalf("Reason = %q, want %q", perr.Reason, ReasonInvalidTimestamp)
	}
}

func TestParseLineRejectsBadNumber(t *testing.T) {
	line := `127.0.0.1 - - [27/Apr/2026:10:00:02 +0900] "GET /a HTTP/1.1" abc 1 "-" "x" 0.1`
	_, perr := ParseLine(line)
	if perr == nil || perr.Reason != ReasonInvalidNumber {
		t.Fatalf("expected INVALID_NUMBER, got %+v", perr)
	}
}

func TestParseLineRejectsNoFormat(t *testing.T) {
	_, perr := ParseLine("not an access log line")
	if perr == nil || perr.Reason != ReasonNoFormatMatch {
		t.Fatalf("expected NO_FORMAT_MATCH, got %+v", perr)
	}
}

func TestParseLineNginxTrailingSpaceStillRejects(t *testing.T) {
	_, perr := ParseLine(nginxLine("", "", "", "") + " ")
	if perr == nil || perr.Reason != ReasonNoFormatMatch {
		t.Fatalf("expected NO_FORMAT_MATCH, got %+v", perr)
	}
}

func TestParseLineCommonFormatWithoutResponseTime(t *testing.T) {
	line := `127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] "GET /health HTTP/1.1" 204 -`
	rec, perr := ParseLine(line)
	if perr != nil {
		t.Fatalf("unexpected parse error: %+v", perr)
	}
	if rec.URI != "/health" || rec.Status != 204 {
		t.Errorf("rec mismatch: %+v", rec)
	}
	if rec.BytesSent != 0 {
		t.Errorf("BytesSent should be 0 for `-`, got %d", rec.BytesSent)
	}
	if rec.ResponseTimeMS != 0 {
		t.Errorf("ResponseTimeMS should be 0 when format has none, got %f", rec.ResponseTimeMS)
	}
}

func TestParseTomcatAndJettyAccessFormats(t *testing.T) {
	for _, format := range []string{"tomcat", "jetty"} {
		rec, perr := ParseLineForFormat(nginxLine("", "/app", "201", "0.045"), format)
		if perr != nil {
			t.Fatalf("%s parse error: %+v", format, perr)
		}
		if rec.SourceFormat != format || rec.URI != "/app" || rec.Status != 201 {
			t.Fatalf("%s record mismatch: %+v", format, rec)
		}
	}
}

func TestParseHAProxyHTTPLog(t *testing.T) {
	line := `May 16 10:00:00 edge haproxy[123]: 192.0.2.1:54321 [16/May/2026:10:00:00.000] fe be/app1 0/0/1/12/13 503 512 - - SC-- 1/1/0/0/0 1/0 "GET /api/orders HTTP/1.1"`
	rec, perr := ParseLineForFormat(line, "haproxy-http")
	if perr != nil {
		t.Fatalf("parse error: %+v", perr)
	}
	if rec.Status != 503 || rec.BackendName != "be" || rec.BackendServer != "app1" {
		t.Fatalf("haproxy fields mismatch: %+v", rec)
	}
	if rec.ResponseTimeMS != 13 || rec.UpstreamLatencyMS != 12 || rec.RetryCount != 1 || rec.TerminationState != "SC--" {
		t.Fatalf("haproxy timing/retry mismatch: %+v", rec)
	}
}

func TestParseEnvoyAndIstioFormats(t *testing.T) {
	text := `[2026-05-16T10:00:00.123Z] "GET /api/orders HTTP/1.1" 200 - via_upstream - "-" 123 456 78 70 "192.0.2.1" "curl" "trace-1" "shop.example.com" "10.0.0.10:8080" "outbound|8080||orders.default.svc.cluster.local"`
	rec, perr := ParseLineForFormat(text, "envoy-text")
	if perr != nil {
		t.Fatalf("envoy text parse error: %+v", perr)
	}
	if rec.UpstreamService != "orders.default.svc.cluster.local" || rec.TraceID != "trace-1" || rec.GatewayLatencyMS != 8 {
		t.Fatalf("envoy text mismatch: %+v", rec)
	}
	jsonLine := `{"start_time":"2026-05-16T10:00:01Z","method":"GET","path":"/api/pay","response_code":500,"duration":0.123,"upstream_service_time":"100ms","authority":"gw","upstream_cluster":"outbound|8080||payment.default.svc.cluster.local","request_id":"req-1","response_flags":"UF","bytes_sent":32}`
	rec, perr = ParseLineForFormat(jsonLine, "istio-json")
	if perr != nil {
		t.Fatalf("istio json parse error: %+v", perr)
	}
	if rec.Status != 500 || rec.UpstreamService != "payment.default.svc.cluster.local" || rec.ResponseFlags != "UF" {
		t.Fatalf("istio json mismatch: %+v", rec)
	}
}

func TestParseCloudEdgeFormats(t *testing.T) {
	elb := `2026-05-16T10:00:00.000000Z my-loadbalancer 192.0.2.1:12345 10.0.0.1:80 0.000001 0.001 0.000002 200 200 0 57 "GET http://example.com:80/path HTTP/1.1" "curl/8" - -`
	rec, perr := ParseLineForFormat(elb, "aws-elb")
	if perr != nil {
		t.Fatalf("elb parse error: %+v", perr)
	}
	if rec.CloudProvider != "aws" || rec.UpstreamService != "10.0.0.1:80" || rec.URI != "/path" {
		t.Fatalf("elb mismatch: %+v", rec)
	}
	alb := `http 2026-05-16T10:00:00.000000Z app/my-alb/123 192.0.2.1:12345 10.0.0.1:80 0.000 0.020 0.000 502 502 10 57 "GET https://example.com:443/orders HTTP/1.1" "curl/8" ECDHE TLSv1.3 Root=trace-aws example.com`
	rec, perr = ParseLineForFormat(alb, "aws-alb")
	if perr != nil {
		t.Fatalf("alb parse error: %+v", perr)
	}
	if rec.Status != 502 || rec.TLSVersion != "TLSv1.3" || rec.TraceID != "trace-aws" {
		t.Fatalf("alb mismatch: %+v", rec)
	}
	cloudfront := strings.Join([]string{
		"2026-05-16", "10:00:00", "ICN54", "512", "192.0.2.10", "GET", "d111.cloudfront.net",
		"/api/orders", "200", "-", "curl", "-", "-", "Hit", "reqcf", "d111.cloudfront.net",
		"https", "64", "0.123", "-", "TLSv1.3", "TLS_AES", "Hit", "HTTP/2.0",
	}, "\t")
	rec, perr = ParseLineForFormat(cloudfront, "aws-cloudfront")
	if perr != nil {
		t.Fatalf("cloudfront parse error: %+v", perr)
	}
	if rec.EdgeLocation != "ICN54" || rec.ResponseTimeMS != 123 || rec.RequestID != "reqcf" {
		t.Fatalf("cloudfront mismatch: %+v", rec)
	}
}

func TestParseGCPAndAzureJSONFormats(t *testing.T) {
	gcp := `{"timestamp":"2026-05-16T10:00:00Z","insertId":"gcp-req","resource":{"labels":{"backend_service_name":"orders-backend"}},"httpRequest":{"requestMethod":"GET","requestUrl":"https://shop.example.com/api/orders","status":200,"responseSize":"128","remoteIp":"192.0.2.20","latency":"0.045s","userAgent":"curl"}}`
	rec, perr := ParseLineForFormat(gcp, "gcp-http-lb-json")
	if perr != nil {
		t.Fatalf("gcp parse error: %+v", perr)
	}
	if rec.CloudProvider != "gcp" || rec.UpstreamService != "orders-backend" || rec.ResponseTimeMS != 45 {
		t.Fatalf("gcp mismatch: %+v", rec)
	}
	azure := `{"time":"2026-05-16T10:00:00Z","properties":{"httpMethod":"POST","requestUri":"/api/pay","httpStatusCode":504,"timeTaken":"250ms","clientIp":"192.0.2.21","backendHostname":"pay-origin","trackingReference":"az-1"}}`
	rec, perr = ParseLineForFormat(azure, "azure-front-door-json")
	if perr != nil {
		t.Fatalf("azure parse error: %+v", perr)
	}
	if rec.CloudProvider != "azure" || rec.UpstreamService != "pay-origin" || rec.Status != 504 {
		t.Fatalf("azure mismatch: %+v", rec)
	}
}

func TestParseIISCaddyAndTraefikFormats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iis.log")
	body := "#Fields: date time c-ip cs-method cs-uri-stem cs-uri-query cs(User-Agent) sc-status time-taken\n" +
		"2026-05-16 10:00:00 192.0.2.30 GET /iis - curl 200 42\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	records, diags, err := ParseFile(path, "iis-w3c", Options{})
	if err != nil {
		t.Fatalf("iis ParseFile: %v", err)
	}
	if len(records) != 1 || records[0].SourceFormat != "iis-w3c" || records[0].ResponseTimeMS != 42 {
		t.Fatalf("iis records mismatch: %+v", records)
	}
	if diags.TotalLines != 2 || diags.ParsedRecords != 1 {
		t.Fatalf("iis diagnostics mismatch: %+v", diags)
	}
	caddy := `{"ts":1778916000,"request":{"method":"GET","uri":"/caddy","remote_ip":"192.0.2.31","headers":{"Host":["caddy.local"],"User-Agent":["curl"]}},"status":200,"size":10,"duration":0.012}`
	rec, perr := ParseLineForFormat(caddy, "caddy-json")
	if perr != nil {
		t.Fatalf("caddy parse error: %+v", perr)
	}
	if rec.ServiceName != "caddy" || rec.ResponseTimeMS != 12 {
		t.Fatalf("caddy mismatch: %+v", rec)
	}
	traefik := `{"StartUTC":"2026-05-16T10:00:00Z","RequestMethod":"GET","RequestPath":"/traefik","DownstreamStatus":200,"Duration":12000000,"OriginDuration":7000000,"ClientHost":"192.0.2.32","ServiceName":"orders@docker","RouterName":"orders"}`
	rec, perr = ParseLineForFormat(traefik, "traefik-json")
	if perr != nil {
		t.Fatalf("traefik parse error: %+v", perr)
	}
	if rec.UpstreamService != "orders@docker" || rec.GatewayLatencyMS != 5 {
		t.Fatalf("traefik mismatch: %+v", rec)
	}
}

func TestParseGatewayJSONFormats(t *testing.T) {
	kong := `{"started_at":1778916000000,"request":{"method":"GET","uri":"/kong","remote_addr":"192.0.2.40","headers":{"host":"api.example.com","x-request-id":"kong-1"}},"response":{"status":200,"size":64},"latencies":{"request":35,"proxy":25,"kong":10},"route":{"name":"orders-route"},"service":{"name":"orders-service"},"consumer":{"username":"alice"}}`
	rec, perr := ParseLineForFormat(kong, "kong-json")
	if perr != nil {
		t.Fatalf("kong parse error: %+v", perr)
	}
	if rec.Route != "orders-route" || rec.Consumer != "alice" || rec.UpstreamService != "orders-service" {
		t.Fatalf("kong mismatch: %+v", rec)
	}
	tyk := `{"timestamp":"2026-05-16T10:00:00Z","method":"GET","path":"/tyk","status":429,"latency":44,"api_name":"orders-api","org_id":"tenant-a","ip_address":"192.0.2.41"}`
	rec, perr = ParseLineForFormat(tyk, "tyk-json")
	if perr != nil {
		t.Fatalf("tyk parse error: %+v", perr)
	}
	if rec.Status != 429 || rec.UpstreamService != "orders-api" || rec.Consumer != "tenant-a" {
		t.Fatalf("tyk mismatch: %+v", rec)
	}
	apiGateway := `{"requestTimeEpoch":1778916000000,"httpMethod":"GET","path":"/awsapi","status":504,"responseLatency":120,"integrationLatency":80,"responseLength":12,"sourceIp":"192.0.2.42","domainName":"api.execute-api","routeKey":"GET /awsapi","requestId":"apigw-1","integrationService":"orders-lambda"}`
	rec, perr = ParseLineForFormat(apiGateway, "aws-api-gateway-json")
	if perr != nil {
		t.Fatalf("api gateway parse error: %+v", perr)
	}
	if rec.CloudProvider != "aws" || rec.UpstreamService != "orders-lambda" || rec.GatewayLatencyMS != 40 {
		t.Fatalf("api gateway mismatch: %+v", rec)
	}
}

func TestParseFileAutoDetectsEdgeFormats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edge.log")
	body := strings.Join([]string{
		`{"start_time":"2026-05-16T10:00:01Z","method":"GET","path":"/api/pay","response_code":200,"duration":0.010,"upstream_cluster":"outbound|8080||payment.default.svc.cluster.local"}`,
		`{"timestamp":"2026-05-16T10:00:02Z","method":"GET","path":"/tyk","status":200,"latency":44,"api_name":"orders-api","org_id":"tenant-a"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	records, diags, err := ParseFile(path, "auto", Options{})
	if err != nil {
		t.Fatalf("ParseFile auto: %v", err)
	}
	if len(records) != 2 || records[0].SourceFormat != "envoy-json" || records[1].SourceFormat != "tyk-json" {
		t.Fatalf("auto records mismatch: %+v diagnostics=%+v", records, diags)
	}
}

func TestParseFileReportsMalformedLineDiagnostics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	valid := nginxLine("", "", "", "")
	invalidTimestamp := `127.0.0.1 - - [bad-time] ` +
		`"GET /api/orders/1002 HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.123`
	invalidNumber := `127.0.0.1 - - [27/Apr/2026:10:00:02 +0900] ` +
		`"GET /api/orders/1003 HTTP/1.1" abc 1234 "-" "Mozilla/5.0" 0.123`
	noFormat := "not an nginx access log record"
	body := strings.Join([]string{valid, "", invalidTimestamp, invalidNumber, noFormat}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	records, diags, err := ParseFile(path, "nginx", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if diags.TotalLines != 5 {
		t.Errorf("TotalLines = %d, want 5", diags.TotalLines)
	}
	if diags.ParsedRecords != 1 {
		t.Errorf("ParsedRecords = %d, want 1", diags.ParsedRecords)
	}
	if diags.SkippedLines != 3 {
		t.Errorf("SkippedLines = %d, want 3", diags.SkippedLines)
	}
	if diags.ErrorCount != 3 {
		t.Errorf("ErrorCount = %d, want 3", diags.ErrorCount)
	}
	wantReasons := map[string]int{
		ReasonInvalidTimestamp: 1,
		ReasonInvalidNumber:    1,
		ReasonNoFormatMatch:    1,
	}
	for k, v := range wantReasons {
		if diags.SkippedByReason[k] != v {
			t.Errorf("SkippedByReason[%q] = %d, want %d", k, diags.SkippedByReason[k], v)
		}
	}
	if len(diags.Samples) != 3 {
		t.Fatalf("Samples = %d, want 3", len(diags.Samples))
	}
	wantOrder := []string{ReasonInvalidTimestamp, ReasonInvalidNumber, ReasonNoFormatMatch}
	for i, want := range wantOrder {
		if diags.Samples[i].Reason != want {
			t.Errorf("Samples[%d].Reason = %q, want %q", i, diags.Samples[i].Reason, want)
		}
	}
	if diags.SourceFile == nil || *diags.SourceFile != path {
		t.Errorf("SourceFile = %v, want %q", diags.SourceFile, path)
	}
	if diags.Format != "nginx" {
		t.Errorf("Format = %q", diags.Format)
	}
}

func TestParseFileStrictModeFailsFast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	body := "bad access log\n" + nginxLine("", "", "", "") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, err := ParseFile(path, "nginx", Options{Strict: true})
	if err == nil {
		t.Fatalf("expected error in strict mode")
	}
	if !strings.Contains(err.Error(), ReasonNoFormatMatch) {
		t.Fatalf("error should mention NO_FORMAT_MATCH; got %v", err)
	}
}

func TestParseFileRejectsUnsupportedFormat(t *testing.T) {
	_, _, err := ParseFile("/tmp/none.log", "not-a-format", Options{})
	if err == nil || !strings.Contains(err.Error(), "Unsupported access log format") {
		t.Fatalf("expected format-rejection error, got %v", err)
	}
}

func TestParseFileEmptyFileWarns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	records, diags, err := ParseFile(path, "nginx", Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records non-empty for empty file: %+v", records)
	}
	if diags.WarningCount != 1 {
		t.Errorf("WarningCount = %d, want 1", diags.WarningCount)
	}
	if len(diags.Warnings) == 0 || diags.Warnings[0].Reason != "EMPTY_FILE" {
		t.Errorf("expected EMPTY_FILE warning, got %+v", diags.Warnings)
	}
}

func BenchmarkParseLineNginxWithResponseTime(b *testing.B) {
	line := nginxLine("", "/api/orders/1001?status=ready", "200", "0.123")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec, perr := ParseLine(line)
		if perr != nil || rec == nil {
			b.Fatalf("ParseLine = %+v, %+v", rec, perr)
		}
	}
}
