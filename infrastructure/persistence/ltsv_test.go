package persistence

import "testing"

func Test_ltsvRepository_labelAndData(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		lr      *ltsvRepository
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name: "get label and data",
			lr:   &ltsvRepository{},
			args: args{
				s: "label:data",
			},
			want:    "label",
			want1:   "data",
			wantErr: false,
		},
		{
			name: "error happen because data with out label",
			lr:   &ltsvRepository{},
			args: args{
				s: "",
			},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name: "error happen because string has only delimiter':'",
			lr:   &ltsvRepository{},
			args: args{
				s: ":",
			},
			want:    "",
			want1:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lr := &ltsvRepository{}
			got, got1, err := lr.labelAndData(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ltsvRepository.labelAndField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ltsvRepository.labelAndField() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ltsvRepository.labelAndField() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
