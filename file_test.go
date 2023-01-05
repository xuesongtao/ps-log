package pslog

import (
	"testing"
	"time"

	"gitee.com/xuesongtao/gotool/base"
	"gitee.com/xuesongtao/gotool/xfile"
)

var (
	fileInfo = &FileInfo{}
)

func TestParse(t *testing.T) {
	fileInfo.Parse("./ps-log/_example/parselog/main.go")
	t.Log(fileInfo.Dir, fileInfo.Name)
	t.Log(fileInfo.FileName())
}

func TestGetContent(t *testing.T) {
	tmp := tmpDir + "/test.log"

	f := &FileInfo{}
	for i := 0; i < 3; i++ {
		_, err := f.putContent(tmp, "line:"+base.ToString(i)+"test"+time.Now().Format(base.DatetimeFmt)+"\n")
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second)

		row, err := f.getContent(tmp)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(row)
	}
	filePool.Close()
	time.Sleep(time.Second * 2)
}

func TestGetContent1(t *testing.T) {
	tmp := tmpDir + "/test.log"

	for i := 0; i < 3; i++ {
		_, err := xfile.PutContent(tmp, "line:"+base.ToString(i)+"test"+time.Now().Format(base.DatetimeFmt)+"\n")
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second)

		row, err := xfile.GetContent(tmp)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(row)
	}
}
