package pslog

import (
	"testing"

	"gitee.com/xuesongtao/gotool/base"
)

func TestTmp(t *testing.T) {
	map1 := make(map[string]*FileInfo, 10)
	map1["tmp/test.log"] = &FileInfo{
		Dir:          "tmp",
		Name:         "test.log",
		offsetChange: 1,
		offset:       2,
	}
	t.Log(base.ToString(map1))
	// map2 := map1
	map2 := make(map[string]*FileInfo, 10)
	for k, v := range map1 {
		map2[k] = v
	}
	// map2["tmp/test.log"] = &FileInfo{
	// 	Dir:          "tmp",
	// 	Name:         "test.log1234",
	// 	offsetChange: 1,
	// 	offset:       20,
	// }
	map2["tmp/test.log"].Name = "test.log122"
	t.Log(base.ToString(map1))
	t.Log(base.ToString(map2))
	delete(map2, "tmp/test.log")
	t.Log(base.ToString(map2))
	t.Log(base.ToString(map1))
	map1 = map2
	t.Log(base.ToString(map1))

}
