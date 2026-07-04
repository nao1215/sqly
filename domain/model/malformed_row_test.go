package model

import "testing"

func TestMalformedRowPolicy_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		policy MalformedRowPolicy
		want   string
	}{
		{name: "stop policy prints stop", policy: MalformedRowStop, want: "stop"},
		{name: "skip policy prints skip", policy: MalformedRowSkip, want: "skip"},
		{name: "fill policy prints fill", policy: MalformedRowFill, want: "fill"},
		{name: "unknown policy falls back to stop", policy: MalformedRowPolicy(99), want: "stop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.policy.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMalformedRowPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    MalformedRowPolicy
		wantErr bool
	}{
		{name: "stop parses to MalformedRowStop", input: "stop", want: MalformedRowStop},
		{name: "skip parses to MalformedRowSkip", input: "skip", want: MalformedRowSkip},
		{name: "fill parses to MalformedRowFill", input: "fill", want: MalformedRowFill},
		{name: "empty string is rejected", input: "", wantErr: true},
		{name: "unknown value is rejected", input: "keep", wantErr: true},
		{name: "uppercase is rejected", input: "STOP", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseMalformedRowPolicy(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseMalformedRowPolicy(%q) expected an error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMalformedRowPolicy(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseMalformedRowPolicy(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseMalformedRowPolicy_RoundTrip is a metamorphic check: every policy's
// String() output parses back to the same policy.
func TestParseMalformedRowPolicy_RoundTrip(t *testing.T) {
	t.Parallel()
	for _, policy := range []MalformedRowPolicy{MalformedRowStop, MalformedRowSkip, MalformedRowFill} {
		got, err := ParseMalformedRowPolicy(policy.String())
		if err != nil {
			t.Fatalf("round-trip of %v failed: %v", policy, err)
		}
		if got != policy {
			t.Errorf("round-trip of %v produced %v", policy, got)
		}
	}
}
