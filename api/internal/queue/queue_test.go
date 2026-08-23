package queue

import "testing"

// The payload shape is the contract with the Python consumer, which
// reads it by key name. Pin it.
func TestIngestJobPayload(t *testing.T) {
	got, err := IngestJob{DocumentID: 42}.payload()
	if err != nil {
		t.Fatalf("payload() returned error: %v", err)
	}

	const want = `{"document_id":42}`
	if string(got) != want {
		t.Errorf("payload() = %s, want %s", got, want)
	}
}

func TestIngestListName(t *testing.T) {
	const want = "knowdo:jobs:ingest"
	if IngestList != want {
		t.Errorf("IngestList = %q, want %q", IngestList, want)
	}
}
