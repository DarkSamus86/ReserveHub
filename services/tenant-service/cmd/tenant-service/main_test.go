package main

import "testing"

func TestReflectionOnlyInLocalEnvironment(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want bool
	}{
		{"local", true},
		{"", false},
		{"staging", false},
		{"production", false},
	} {
		if got := reflectionEnabled(tc.env); got != tc.want {
			t.Errorf("reflectionEnabled(%q) = %t, want %t", tc.env, got, tc.want)
		}
	}
}
