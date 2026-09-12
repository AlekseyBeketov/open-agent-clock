package provider

import "testing"

func TestClassifyUsageLimitWithoutRetry(t *testing.T) {
	code := 1
	status, reason := (cliAdapter{}).Classify(&code, false, "", "rate limit reached")
	if status != StatusUsageLimited || reason == "" {
		t.Fatalf("status=%q reason=%q", status, reason)
	}
}

func TestClassifySuccessDoesNotClaimReset(t *testing.T) {
	code := 0
	status, reason := (cliAdapter{}).Classify(&code, false, "ok", "")
	if status != StatusSuccess {
		t.Fatalf("status=%q", status)
	}
	if reason == "" || reason == "reset" {
		t.Fatalf("unsafe success reason=%q", reason)
	}
}
