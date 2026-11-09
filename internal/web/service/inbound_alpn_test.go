package service

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCanonicalizeWebSocketALPN_PreservesUserChoice(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantALPN []string
	}{
		{
			name:     "empty alpn defaults to http/1.1",
			input:    `{"network":"ws","security":"tls","tlsSettings":{}}`,
			wantALPN: []string{"http/1.1"},
		},
		{
			name:     "explicit h2 is preserved",
			input:    `{"network":"ws","security":"tls","tlsSettings":{"alpn":["h2"]}}`,
			wantALPN: []string{"h2"},
		},
		{
			name:     "explicit h2 and http/1.1 preserved",
			input:    `{"network":"ws","security":"tls","tlsSettings":{"alpn":["h2","http/1.1"]}}`,
			wantALPN: []string{"h2", "http/1.1"},
		},
		{
			name:     "explicit h3, h2, http/1.1 preserved",
			input:    `{"network":"ws","security":"tls","tlsSettings":{"alpn":["h3","h2","http/1.1"]}}`,
			wantALPN: []string{"h3", "h2", "http/1.1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalizeWebSocketALPN(tc.input)
			var stream map[string]any
			if err := json.Unmarshal([]byte(got), &stream); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			tls := stream["tlsSettings"].(map[string]any)
			rawAlpn := tls["alpn"].([]any)
			var gotALPN []string
			for _, a := range rawAlpn {
				gotALPN = append(gotALPN, a.(string))
			}
			if !reflect.DeepEqual(gotALPN, tc.wantALPN) {
				t.Errorf("got %v, want %v", gotALPN, tc.wantALPN)
			}
		})
	}
}
