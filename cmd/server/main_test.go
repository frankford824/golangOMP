package main

import "testing"

func TestEnvFlagAcceptsDeploymentBooleanValues(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "one", raw: "1", want: true},
		{name: "true", raw: "true", want: true},
		{name: "yes", raw: "yes", want: true},
		{name: "on", raw: "on", want: true},
		{name: "false", raw: "false", want: false},
		{name: "empty", raw: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENABLE_CRON_TEST", tc.raw)
			if got := envFlag("ENABLE_CRON_TEST"); got != tc.want {
				t.Fatalf("envFlag() = %v, want %v", got, tc.want)
			}
		})
	}
}
