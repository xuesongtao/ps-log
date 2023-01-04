package pslog

import (
	"path/filepath"
	"testing"
)

func TestTmp(t *testing.T) {
	t.Log(filepath.Join("test", "a", "b.txt", "b"))
}
