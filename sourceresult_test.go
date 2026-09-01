package netscope

import "testing"

func TestSourceResultZeroValue(t *testing.T) {
	var r SourceResult
	if r.Status != Ran {
		t.Errorf("zero-value Status = %v, want Ran", r.Status)
	}
	if r.Source != "" || r.Reason != "" {
		t.Errorf("zero-value SourceResult has non-empty strings: %+v", r)
	}
}

func TestSourceResultUnavailableCarriesReason(t *testing.T) {
	r := SourceResult{Source: "kernel", Status: Unavailable, Reason: "no CAP_NET_RAW"}
	if r.Status != Unavailable {
		t.Fatalf("Status = %v, want Unavailable", r.Status)
	}
	if r.Reason == "" {
		t.Fatal("Reason must be populated when Status is Unavailable")
	}
}
