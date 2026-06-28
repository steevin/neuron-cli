package gitsync

import "testing"

func TestIsRemoteURL(t *testing.T) {
	cases := map[string]bool{
		"origin":                         false,
		"backup":                         false,
		"https://example.com/repo.git":   true,
		"ssh://git@example.com/repo.git": true,
		"git@example.com:org/repo.git":   true,
		"git@github.com:org/repo.git":    true,
		"file:///tmp/neuron-remote.git":  true,
		"/tmp/neuron-remote.git":         true,
		"../neuron-remote.git":           true,
		"./neuron-remote.git":            true,
	}

	for remote, want := range cases {
		if got := isRemoteURL(remote); got != want {
			t.Fatalf("isRemoteURL(%q) = %v, want %v", remote, got, want)
		}
	}
}
