package pslog

import (
	"testing"

	"gitee.com/xuesongtao/gotool/base"
)

func TestParseLogPath(t *testing.T) {
	obj := NewLogPath()
	obj.SetExcludeProjectDir("tmp/project/test1.log")
	path := make([]*ProjectLog, 0)
	res := obj.ParseLogPath(&Project{
		LogDir: &LogDir{
			Name:        "",
			TargetNames: []string{"test.log"},
			Namespace:   "namespace_01",
		},
		ProjectPath: "/Users/xuesongtao/goProject/src/myGo/ps-log/tmp/project",
	})
	path = append(path, res...)
	t.Log(base.ToString(path))
}
