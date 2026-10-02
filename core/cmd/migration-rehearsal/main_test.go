package main

import "testing"

func TestValidateRehearsalDSNUsesEffectivePGXTarget(t *testing.T) {
	const base = "postgres://postgres:example@127.0.0.1:54321/fortuna_rehearsal_test?sslmode=disable"
	for _, tc := range []struct {
		name string
		dsn  string
		want bool
	}{
		{name: "isolated target", dsn: base, want: true},
		{name: "host query override", dsn: base + "&host=remote.example"},
		{name: "database query override", dsn: base + "&dbname=production"},
		{name: "port query override", dsn: base + "&port=5432"},
		{name: "non-loopback URL", dsn: "postgres://postgres:example@remote.example:54321/fortuna_rehearsal_test?sslmode=disable"},
		{name: "unmarked database", dsn: "postgres://postgres:example@127.0.0.1:54321/production?sslmode=disable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateRehearsalDSN(tc.dsn) == nil; got != tc.want {
				t.Fatalf("accepted = %t, want %t", got, tc.want)
			}
		})
	}
}
