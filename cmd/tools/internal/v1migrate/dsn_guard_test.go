package v1migrate

import "testing"

func TestGuardR35DSN(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{
			name:    "production database rejected",
			dsn:     "u:p@tcp(127.0.0.1:3306)/jst_erp?parseTime=true",
			wantErr: true,
		},
		{
			name:    "r35 test database accepted",
			dsn:     "u:p@tcp(127.0.0.1:3306)/jst_erp_r3_test?parseTime=true&multiStatements=true",
			wantErr: false,
		},
		{
			name:    "malformed dsn rejected",
			dsn:     "://bad-dsn",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GuardR35DSN(tt.dsn)
			if tt.wantErr && err == nil {
				t.Fatalf("GuardR35DSN(%q) error = nil, want error", tt.dsn)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("GuardR35DSN(%q) error = %v, want nil", tt.dsn, err)
			}
		})
	}
}
