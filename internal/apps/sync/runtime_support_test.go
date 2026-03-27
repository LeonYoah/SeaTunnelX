package sync

import "testing"

func TestUnwrapPrecheckPayloadReturnsInnerMessage(t *testing.T) {
	raw := `{"success":true,"message":"{\"running\":false,\"status\":\"success\",\"exit_code\":0}"}`
	got := unwrapPrecheckPayload(raw)
	want := `{"running":false,"status":"success","exit_code":0}`
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestUnwrapPrecheckPayloadFallsBackToOriginalOutput(t *testing.T) {
	raw := `{"running":true,"status":"running"}`
	got := unwrapPrecheckPayload(raw)
	if got != raw {
		t.Fatalf("expected raw output, got %s", got)
	}
}
