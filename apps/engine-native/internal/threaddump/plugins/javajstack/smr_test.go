package javajstack

import "testing"

// T-228 / T-234 — SMR diagnostics.

func TestSMRZombieMarkerExtractsUnresolved(t *testing.T) {
	lines := []string{
		"Threads class SMR info:",
		"  unresolved zombie thread 0x000000001234abcd",
	}
	smr := parseSMRDiagnostics(lines)
	if smr == nil {
		t.Fatal("smr payload should not be nil")
	}
	if smr["unresolved_count"] != 1 {
		t.Fatalf("unresolved_count = %v", smr["unresolved_count"])
	}
	addrs := smr["addresses"].([]map[string]any)
	if len(addrs) != 1 {
		t.Fatalf("addresses length = %d", len(addrs))
	}
	if addrs[0]["address"] != "0x1234abcd" {
		t.Fatalf("normalized address = %v", addrs[0]["address"])
	}
	if addrs[0]["tagged_unresolved"] != true {
		t.Fatalf("tagged_unresolved = %v", addrs[0]["tagged_unresolved"])
	}
}

func TestSMRJDK21StyleResolvesAgainstParsedTids(t *testing.T) {
	lines := []string{
		"Threads class SMR info:",
		"_java_thread_list=0x00007f8a0001a000, length=2",
		"=>0x000000001234abcd",
		"  0x00007f8a0099ee01",
	}
	smr := parseSMRDiagnostics(lines)
	if smr == nil {
		t.Fatal("smr payload nil")
	}
	if smr["length"] != 2 {
		t.Fatalf("length = %v", smr["length"])
	}
	records := []threadDumpRecord{
		{ThreadName: "main", ThreadID: "0x000000001234abcd"},
	}
	postProcessSMR(smr, records)

	if smr["resolved_count"] != 1 {
		t.Fatalf("resolved_count = %v", smr["resolved_count"])
	}
	resolved := smr["resolved"].([]map[string]any)
	if resolved[0]["thread_name"] != "main" {
		t.Fatalf("resolved thread_name = %v", resolved[0]["thread_name"])
	}
	if smr["addresses_unresolved_count"] != 1 {
		t.Fatalf("addresses_unresolved_count = %v", smr["addresses_unresolved_count"])
	}
	unresolved := smr["addresses_unresolved"].([]map[string]any)
	if unresolved[0]["address"] != "0x7f8a0099ee01" {
		t.Fatalf("unresolved address = %v", unresolved[0]["address"])
	}
}

func TestSMRJDK8FiltersJVMThreadList(t *testing.T) {
	lines := []string{
		"Threads class SMR info:",
		"_java_thread_list=0x00007fff10000000, length=2",
		"  0x00007fff10001000",
		"  0x00007fff10002000",
	}
	smr := parseSMRDiagnostics(lines)
	addrs := smr["addresses"].([]map[string]any)
	for _, a := range addrs {
		if a["address"] == "0x7fff10000000" {
			t.Fatalf("_java_thread_list address should NOT be in addresses list")
		}
	}
	records := []threadDumpRecord{
		{ThreadName: "main", ThreadID: "0x00007fff10001000"},
	}
	postProcessSMR(smr, records)
	if smr["resolved_count"] != 1 {
		t.Fatalf("resolved_count = %v", smr["resolved_count"])
	}
	unresolved := smr["addresses_unresolved"].([]map[string]any)
	addresses := map[string]bool{}
	for _, u := range unresolved {
		addresses[u["address"].(string)] = true
	}
	if !addresses["0x7fff10002000"] {
		t.Fatalf("expected 0x7fff10002000 in unresolved addresses, got %v", addresses)
	}
	if addresses["0x7fff10000000"] {
		t.Fatalf("_java_thread_list bookkeeping address inflated unresolved")
	}
}

func TestSMRDuplicateAddressCountedOnce(t *testing.T) {
	lines := []string{
		"Threads class SMR info:",
		"=>0x0000000012345678",
		"  0x0000000012345678",
	}
	smr := parseSMRDiagnostics(lines)
	postProcessSMR(smr, nil)
	if smr["addresses_unresolved_count"] != 1 {
		t.Fatalf("dedup failed: %v", smr["addresses_unresolved_count"])
	}
}

func TestSMRNoBlockReturnsNil(t *testing.T) {
	lines := []string{
		"Some unrelated text",
		"no smr here",
	}
	if smr := parseSMRDiagnostics(lines); smr != nil {
		t.Fatalf("expected nil, got %+v", smr)
	}
}

func TestNormalizeSMRAddressLowersAndStripsZeros(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0x000000001234ABCD", "0x1234abcd"},
		{"0X00FF", "0xff"},
		{"0x00", "0x0"},
		{"deadbeef", "deadbeef"},
	}
	for _, c := range cases {
		if got := normalizeSMRAddress(c.in); got != c.want {
			t.Errorf("normalizeSMRAddress(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
