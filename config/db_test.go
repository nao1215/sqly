package config

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestNewInMemDB(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "generate new memory db",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cleanup, err := NewInMemDB()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewInMemDB() error = %v, wantErr %v", err, tt.wantErr)
			}
			cleanup()
		})
	}
}

func TestNewHistoryDB(t *testing.T) {
	type args struct {
		c *Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "generate new history db",
			args: args{
				c: &Config{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cleanup, err := NewHistoryDB(tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewHistoryDB() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			cleanup()
		})
	}
}
