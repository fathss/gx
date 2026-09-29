package git

import "testing"

func TestQualify(t *testing.T) {
	tests := []struct {
		name   string
		remote string
		branch string
		want   string
	}{
		{"qualifies bare name", "origin", "main", "origin/main"},
		{"idempotent on qualified ref", "origin", "origin/main", "origin/main"},
		{"nested branch name", "origin", "feat/login", "origin/feat/login"},
		{"qualified nested stays", "origin", "origin/feat/login", "origin/feat/login"},
		{"empty remote returns branch", "", "main", "main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Qualify(tt.remote, tt.branch); got != tt.want {
				t.Errorf("Qualify(%q, %q) = %q, want %q", tt.remote, tt.branch, got, tt.want)
			}
		})
	}
}

func TestStripRemote(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{"strips prefix", "origin/main", "main"},
		{"strips nested", "origin/feat/login", "feat/login"},
		{"bare name unchanged", "main", "main"},
		{"foreign remote unchanged", "fork/main", "fork/main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripRemote("origin", tt.ref); got != tt.want {
				t.Errorf("StripRemote(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}
