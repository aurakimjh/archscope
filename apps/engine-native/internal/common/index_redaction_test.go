package common

import (
	"strings"
	"testing"
)

func TestR1203IndexPolicyAndAuthorizationIdempotency(t *testing.T) {
	aws := "AKIA" + strings.Repeat("A", 16)
	jwt := "eyJzeW50aGV0aWMiOiJmaXh0dXJlIn0.eyJ0ZXN0Ijp0cnVlfQ.c3ludGhldGlj"
	for _, value := range []string{"password=synthetic-secret", `{"api_key":"synthetic-secret"}`, "service.access_key=synthetic-secret", jwt, aws, "Authorization: Bearer synthetic-secret", "Authorization: Basic synthetic-secret"} {
		out := RedactIndexText(value)
		if strings.Contains(out, "synthetic-secret") || strings.Contains(out, jwt) || strings.Contains(out, aws) || out == value {
			t.Fatal("index secret retained", out)
		}
		if again := RedactIndexText(out); again != out {
			t.Fatalf("reapplication changed mask %q -> %q", out, again)
		}
	}
	legacy := RedactText("password=synthetic-secret").Text
	if legacy != "password=synthetic-secret" || RedactionVersion != "0.1.0" || len(IndexRedactionSHA256) != 64 {
		t.Fatal("legacy capture policy changed")
	}
}
