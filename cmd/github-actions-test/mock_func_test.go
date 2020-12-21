package mock_func

import (
	"testing"
)

func TestAbs(t *testing.T) {
	a := Abs(-1)
	b := Abs(1)
	if a != b {
		t.Errorf("Abs(-1) != Abs(1)")
	}
}
