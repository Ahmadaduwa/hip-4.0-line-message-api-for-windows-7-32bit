package db

import (
	"database/sql"
	"strings"
	"testing"

	"stw_teacher_check/config"
)

func TestBuildConnStringReal(t *testing.T) {
	tests := []struct {
		name       string
		cfg        config.Settings
		expectedIn []string
	}{
		{
			name: "Windows Auth default",
			cfg: config.Settings{
				DBHost:   `.\\SQLEXPRESS`,
				DBName:   "hiptime",
				DBAuth:   "windows",
				DBPort:   1433,
				MockMode: false,
			},
			expectedIn: []string{`server=.\SQLEXPRESS`, `database=hiptime`, `trusted_connection=yes`},
		},
		{
			name: "Windows Auth quad slash",
			cfg: config.Settings{
				DBHost:   `.\\\\SQLEXPRESS`,
				DBName:   "hiptime",
				DBAuth:   "windows",
				DBPort:   1433,
				MockMode: false,
			},
			expectedIn: []string{`server=.\SQLEXPRESS`, `database=hiptime`, `trusted_connection=yes`},
		},
		{
			name: "SQL Auth sa",
			cfg: config.Settings{
				DBHost:     "127.0.0.1",
				DBPort:     1433,
				DBName:     "hiptime",
				DBAuth:     "sql",
				DBUser:     "sa",
				DBPassword: "secretPassword",
				MockMode:   false,
			},
			expectedIn: []string{`server=127.0.0.1`, `database=hiptime`, `user id=sa`, `password=secretPassword`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connStr := buildConnString(tt.cfg)
			t.Logf("Generated connStr: %s", connStr)

			for _, exp := range tt.expectedIn {
				if !strings.Contains(connStr, exp) {
					t.Errorf("expected %q in connStr, got: %s", exp, connStr)
				}
			}

			db, err := sql.Open("sqlserver", connStr)
			if err != nil {
				t.Fatalf("sql.Open failed: %v", err)
			}
			defer db.Close()

			err = db.Ping()
			t.Logf("Ping result: %v", err)
			if err != nil && strings.Contains(err.Error(), "invalid URL escape") {
				t.Fatalf("Ping failed with invalid URL escape error: %v", err)
			}
		})
	}
}

func TestParseDateTime(t *testing.T) {
	tests := []struct {
		input       string
		expectedDay string
		expectedHour int
		expectedMin int
	}{
		{"2026-10-23 09:43:33 AM", "2026-10-23", 9, 43},
		{"2026-10-23 17:53:14 PM", "2026-10-23", 17, 53},
		{"2569-10-25 09:06:33 AM", "2026-10-25", 9, 6},
		{"2026-10-25 09:07:44.177", "2026-10-25", 9, 7},
		{"2026/10/23 07:30:00", "2026-10-23", 7, 30},
	}

	for _, tt := range tests {
		parsed := parseDateTime(tt.input)
		if parsed.IsZero() {
			t.Fatalf("failed to parse: %s", tt.input)
		}
		if parsed.Format("2006-01-02") != tt.expectedDay {
			t.Errorf("input %s: expected day %s, got %s", tt.input, tt.expectedDay, parsed.Format("2006-01-02"))
		}
		if parsed.Hour() != tt.expectedHour || parsed.Minute() != tt.expectedMin {
			t.Errorf("input %s: expected %02d:%02d, got %02d:%02d", tt.input, tt.expectedHour, tt.expectedMin, parsed.Hour(), parsed.Minute())
		}
	}
}
