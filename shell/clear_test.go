package shell

import (
	"context"
	"testing"
)

func Test_clearCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "clear command executes without error",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := NewCommands()
			s := &Shell{}
			err := c.clearCommand(context.Background(), s, []string{})

			if (err != nil) != tt.wantErr {
				t.Errorf("clearCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
