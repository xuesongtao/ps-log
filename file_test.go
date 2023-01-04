package pslog

import (
	"testing"
)

var (
	fileInfo = &FileInfo{}
)

func TestParse(t *testing.T) {
	fileInfo.Parse("./ps-log/_example/parselog/main.go")
	t.Log(fileInfo.Dir, fileInfo.Name)
	t.Log(fileInfo.FileName())
}
