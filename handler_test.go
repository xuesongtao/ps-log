package pslog

import (
	"os"
	"testing"
)

func TestTmp(t *testing.T) {
	t.Log(os.MkdirAll("tmp/.offset/test.log", 0666))
}
