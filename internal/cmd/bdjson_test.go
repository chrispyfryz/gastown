package cmd

import "testing"

func TestDecodeBDJSON_StripsLeadingWarning(t *testing.T) {
	out := []byte("⚠️  Staleness check skipped (--allow-stale), data may be out of sync\n[{\"status\":\"open\"}]\n")
	var v []map[string]string
	if err := decodeBDJSON(out, &v); err != nil {
		t.Fatalf("decodeBDJSON failed: %v", err)
	}
	if len(v) != 1 {
		t.Fatalf("len(v) = %d, want 1", len(v))
	}
	if v[0]["status"] != "open" {
		t.Fatalf("v[0][\"status\"] = %q, want %q", v[0]["status"], "open")
	}
}

func TestDecodeBDJSON_Empty(t *testing.T) {
	var v any
	if err := decodeBDJSON(nil, &v); err == nil {
		t.Fatalf("expected error for empty output")
	}
}
