package adapter

import (
	"encoding/json"
	"strings"
	"testing"
)

func validRecord() Record {
	return Record{
		Schema:     SchemaV1,
		Source:     Source{Kind: "codex", Name: "Codex Sessions"},
		Collection: Collection{ExternalID: "codex:session:s", Kind: "agent_session", Name: "s"},
		Item:       Item{ExternalID: "codex:item:1", Kind: "message"},
	}
}

func TestValidateAcceptsCompleteRecord(t *testing.T) {
	if err := validRecord().Validate(); err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}
}

func TestValidateRejectsMissingFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Record)
		want   string
	}{
		{"bad schema", func(r *Record) { r.Schema = "other" }, "unsupported schema"},
		{"missing source kind", func(r *Record) { r.Source.Kind = "" }, "missing source.kind"},
		{"missing collection external id", func(r *Record) { r.Collection.ExternalID = "" }, "missing collection.external_id"},
		{"missing collection kind", func(r *Record) { r.Collection.Kind = "" }, "missing collection.kind"},
		{"missing item external id", func(r *Record) { r.Item.ExternalID = "" }, "missing item.external_id"},
		{"missing item kind", func(r *Record) { r.Item.Kind = "" }, "missing item.kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := validRecord()
			tc.mutate(&rec)
			err := rec.Validate()
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestParseRoundTripPreservesRawAndValidates(t *testing.T) {
	rec := validRecord()
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(b)
	if err != nil {
		t.Fatalf("Parse rejected valid record: %v", err)
	}
	if parsed.Source.Kind != "codex" {
		t.Fatalf("source kind = %q", parsed.Source.Kind)
	}
	if len(parsed.Unknown) == 0 {
		t.Fatalf("Parse did not preserve raw bytes")
	}
	if string(parsed.Unknown) != string(b) {
		t.Fatalf("Unknown bytes differ from input")
	}
}

func TestParseRejectsMalformedJSON(t *testing.T) {
	if _, err := Parse([]byte("{not json")); err == nil {
		t.Fatalf("expected error for malformed JSON")
	}
}

func TestParseRejectsValidJSONFailingContract(t *testing.T) {
	if _, err := Parse([]byte(`{"schema":"miseledger.adapter.v1"}`)); err == nil {
		t.Fatalf("expected contract validation error for incomplete record")
	}
}
