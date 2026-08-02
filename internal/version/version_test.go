package version

import "testing"

func TestGet(t *testing.T) {
	t.Parallel()
	info := Get()
	if info.Version == "" {
		t.Errorf("expected non-empty version")
	}
}
