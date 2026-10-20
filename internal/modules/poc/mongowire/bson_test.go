package mongowire

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDocumentRoundTrip(t *testing.T) {
	doc := Document{
		{Key: "hello", Value: int32(1)},
		{Key: "name", Value: "arl"},
		{Key: "count", Value: int64(42)},
		{Key: "ratio", Value: 1.5},
		{Key: "enabled", Value: true},
		{Key: "missing", Value: nil},
		{Key: "payload", Value: Binary{Subtype: 0, Data: []byte("abc")}},
		{Key: "nested", Value: Document{{Key: "x", Value: int32(2)}}},
		{Key: "items", Value: []any{"a", int32(3), Document{{Key: "y", Value: true}}}},
	}
	raw, err := EncodeDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got["hello"] != int32(1) || got["name"] != "arl" || got["count"] != int64(42) || got["ratio"] != 1.5 || got["enabled"] != true {
		t.Fatalf("unexpected scalar values: %#v", got)
	}
	if got["missing"] != nil {
		t.Fatalf("missing = %#v", got["missing"])
	}
	bin, ok := got["payload"].(Binary)
	if !ok || !bytes.Equal(bin.Data, []byte("abc")) {
		t.Fatalf("payload = %#v", got["payload"])
	}
	if !reflect.DeepEqual(got["items"], []any{"a", int32(3), map[string]any{"y": true}}) {
		t.Fatalf("items = %#v", got["items"])
	}
}

func TestDecodeRejectsTruncatedDocument(t *testing.T) {
	if _, err := DecodeDocument([]byte{10, 0, 0, 0, 1}); err == nil {
		t.Fatal("expected truncated document to fail")
	}
}

func TestMongoPasswordDigest(t *testing.T) {
	if got := mongoPasswordDigest("user", "pass"); got != "e0c4a7b97d4db31f5014e9694e567d6b" {
		t.Fatalf("mongoPasswordDigest = %q", got)
	}
}

func TestPreferredMechanismsOrder(t *testing.T) {
	got := preferredMechanisms([]string{"SCRAM-SHA-1", "SCRAM-SHA-256", "MONGODB-X509"})
	if !reflect.DeepEqual(got, []string{"SCRAM-SHA-256", "SCRAM-SHA-1"}) {
		t.Fatalf("preferredMechanisms = %v", got)
	}
}
