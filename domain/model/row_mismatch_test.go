package model

import "testing"

func TestRowMismatchPolicy_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		policy RowMismatchPolicy
		want   string
	}{
		{name: "error policy prints error", policy: RowMismatchError, want: "error"},
		{name: "skip policy prints skip", policy: RowMismatchSkip, want: "skip"},
		{name: "pad policy prints pad", policy: RowMismatchPad, want: "pad"},
		{name: "unknown policy falls back to error", policy: RowMismatchPolicy(99), want: "error"},
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

func TestParseRowMismatchPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    RowMismatchPolicy
		wantErr bool
	}{
		{name: "error parses to RowMismatchError", input: "error", want: RowMismatchError},
		{name: "skip parses to RowMismatchSkip", input: "skip", want: RowMismatchSkip},
		{name: "pad parses to RowMismatchPad", input: "pad", want: RowMismatchPad},
		{name: "the pre-v1.0.0 fill name is rejected", input: "fill", wantErr: true},
		{name: "the pre-v1.0.0 stop name is rejected", input: "stop", wantErr: true},
		{name: "empty string is rejected", input: "", wantErr: true},
		{name: "unknown value is rejected", input: "keep", wantErr: true},
		{name: "uppercase is rejected", input: "ERROR", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRowMismatchPolicy(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseRowMismatchPolicy(%q) expected an error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRowMismatchPolicy(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseRowMismatchPolicy(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseRowMismatchPolicy_RoundTrip is a metamorphic check: every policy's
// String() output parses back to the same policy.
func TestParseRowMismatchPolicy_RoundTrip(t *testing.T) {
	t.Parallel()
	for _, policy := range []RowMismatchPolicy{RowMismatchError, RowMismatchSkip, RowMismatchPad} {
		got, err := ParseRowMismatchPolicy(policy.String())
		if err != nil {
			t.Fatalf("round-trip of %v failed: %v", policy, err)
		}
		if got != policy {
			t.Errorf("round-trip of %v produced %v", policy, got)
		}
	}
}
