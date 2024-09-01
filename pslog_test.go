package pslog

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitee.com/xuesongtao/gotool/base"
	"gitee.com/xuesongtao/gotool/xfile"
	plg "gitee.com/xuesongtao/ps-log/log"
)

var (
	tmpDir = "./tmp"
)

type StrBuf struct {
	Buf strings.Builder
}

func (s *StrBuf) WriteTo(bus *LogHandlerBus) {
	s.Buf.WriteString(bus.Msg)
}

type BytesBuf struct {
	Buf bytes.Buffer
}

func (b *BytesBuf) WriteTo(bus *LogHandlerBus) {
	b.Buf.WriteString(bus.Msg)
}

func TestList(t *testing.T) {
	ps, _ := NewPsLog()
	defer ps.Close()

	handler := &Handler{
		CleanOffset: true,
		Change:      -1,       // 每次都持久化 offset
		Tail:        true,     // 实时监听
		ExpireAt:    NoExpire, // 文件句柄不过期
		Targets: []*Target{
			{
				Content:  "[ERRO]",
				Excludes: []string{"no rows in result set", "request params invaild", "cuxjswvirbg0", "ascriptId has no ew_account"},
				To:       []PsLogWriter{&Stdout{}},
			},
			{
				Content:  "1 ",
				Excludes: []string{"test"},
				To:       []PsLogWriter{&Stdout{}},
			},
			{
				Content:  "2 ",
				Excludes: []string{"test2"},
				To:       []PsLogWriter{&Stdout{}},
			},
		},
	}
	if err := ps.Register(handler); err != nil {
		t.Fatal(err)
	}
	if err := ps.AddPaths(tmpDir + "/test2tail.log"); err != nil {
		t.Fatal(err)
	}
	if err := ps.AddPaths(tmpDir + "/test2cron.log"); err != nil {
		t.Fatal(err)
	}
	t.Log(ps.List())
}

func TestTail(t *testing.T) {
	ps, _ := NewPsLog()
	defer ps.Close()
	ps.TailLogs()

	strBuf := new(StrBuf)
	byteBuf := new(BytesBuf)
	stdout := new(Stdout)
	tmp := tmpDir + "/test2tail.log"
	handler := &Handler{
		CleanOffset: true,
		Change:      -1,       // 每次都持久化 offset
		Tail:        true,     // 实时监听
		ExpireAt:    NoExpire, // 文件句柄不过期
		Targets: []*Target{
			{
				Content:  "warning",
				Excludes: []string{},
				To:       []PsLogWriter{strBuf, byteBuf, stdout},
			},
		},
	}
	if err := ps.Register(handler); err != nil {
		t.Fatal(err)
	}
	if err := ps.AddPaths(tmp); err != nil {
		t.Fatal(err)
	}
	closeCh := make(chan struct{})
	go func() {
		fh := xfile.NewFileHandle(tmp)
		if err := fh.Initf(os.O_WRONLY | os.O_TRUNC); err != nil {
			plg.Error(err)
			return
		}
		defer fh.Close()

		f := fh.GetFile()
		for i := 0; i < 10; i++ {
			// time.Sleep(time.Microsecond)
			_, err := f.WriteString(`{"level":"warning","msg":"q","time":"2024-02-02 17:57:10.399"}`)
			if err != nil {
				plg.Error("write err:", err)
			}
		}
		time.Sleep(time.Second * 2)
		close(closeCh)
	}()

	for range closeCh {
	}

	data, err := xfile.GetContent(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if byteBuf.Buf.String() != strBuf.Buf.String() && byteBuf.Buf.String() != data {
		t.Error("data:", data)
		t.Error("byteBuf:", byteBuf.Buf.String())
		t.Error("strBuf:", strBuf.Buf.String())
	}
}

func TestTailLogSplit(t *testing.T) {
	filepath.Walk(tmpDir, func(path string, info fs.FileInfo, err error) error {
		if strings.Contains(path, "2024-09-01") {
			os.Remove(path)
		}
		return nil
	})
	time.Sleep(time.Second*3)

	ps, _ := NewPsLog()
	defer ps.Close()
	err := ps.TailLogs()
	if err != nil {
		t.Fatal(err)
	}

	strBuf := new(StrBuf)
	byteBuf := new(BytesBuf)
	stdout := new(Stdout)
	handler := &Handler{
		CleanOffset: true,
		Change:      -1,       // 每次都持久化 offset
		Tail:        true,     // 实时监听
		ExpireAt:    NoExpire, // 文件句柄不过期
		Targets: []*Target{
			{
				Content:  "warning",
				Excludes: []string{},
				To:       []PsLogWriter{strBuf, byteBuf, stdout},
			},
		},
		NeedCollect: func(filename string) bool { return strings.Contains(filename, "2024-09-01.log") },
	}

	if err := ps.AddPath2Handler(tmpDir, handler); err != nil {
		t.Fatal(err)
	}
	closeCh := make(chan struct{})
	go func() {
		tmp := tmpDir + "/2024-09-01.log"
		xfile.AppendContent(tmp, "warning 文件分割之前, 开始"+"\n")
		xfile.AppendContent(tmp, "warning 文件分割之前, 1111"+"\n")
		xfile.AppendContent(tmp, "warning 文件分割之前, 2222"+"\n")

		// 模拟日志分割
		os.Rename(tmp, tmpDir + "/2024-09-01.1.log")
		// time.Sleep(time.Second)
		// xfile.AppendContent(newLog, "warning 文件分割之后, 1111"+"\n")
		// xfile.AppendContent(newLog, "warning 文件分割之后, 2222"+"\n")

		// 继续添加
		xfile.AppendContent(tmp, "warning 文件分割之后又开始, 开始"+"\n")
		xfile.AppendContent(tmp, "warning 文件分割之后又开始, 1111"+"\n")
		xfile.AppendContent(tmp, "warning 文件分割之后又开始, 2222"+"\n")
		time.Sleep(time.Second * 10)
		// ps.Close()
		close(closeCh)
	}()

	for range closeCh {
	}

	ps.List()
}

func TestTailLoop(t *testing.T) {
	ps, _ := NewPsLog()
	defer ps.Close()
	if err := ps.TailLogs(); err != nil {
		t.Fatal(err)
	}
	strBuf := new(StrBuf)
	byteBuf := new(BytesBuf)
	stdout := new(Stdout)
	tmp := tmpDir + "/test2tailloop.log"
	handler := &Handler{
		LoopParse:   true,
		CleanOffset: true,
		Change:      -1,       // 每次都持久化 offset
		Tail:        true,     // 实时监听
		ExpireAt:    NoExpire, // 文件句柄不过期
		Targets: []*Target{
			{
				Content:  " ",
				Excludes: []string{},
				To:       []PsLogWriter{strBuf, byteBuf, stdout},
			},
		},
	}
	if err := ps.Register(handler); err != nil {
		t.Fatal(err)
	}
	if err := ps.AddPaths(tmp); err != nil {
		t.Fatal("err:", err)
	}

	closeCh := make(chan struct{})
	go func() {
		fh := xfile.NewFileHandle(tmp)
		if err := fh.Initf(os.O_WRONLY | os.O_TRUNC); err != nil {
			plg.Error(err)
			return
		}
		defer fh.Close()

		for i := 1; i < 5; i++ {
			for j := 0; j < 10/i; j++ {
				// time.Sleep(time.Microsecond)
				_, err := fh.GetFile().WriteString(time.Now().Format(base.DatetimeFmt+".000") + " " + fmt.Sprintf("%d-%d", i, j) + "\n")
				if err != nil {
					plg.Error("write err:", err)
				}
			}
			time.Sleep(time.Second * 5)
			_, err := fh.PutContent("")
			if err != nil {
				plg.Error("err:", err)
			}
			t.Log("reset")
		}
		time.Sleep(time.Second * 5)

		close(closeCh)
	}()

	for range closeCh {
	}

	data, err := xfile.GetContent(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if byteBuf.Buf.String() != strBuf.Buf.String() && byteBuf.Buf.String() != data {
		t.Error("data:", data)
		t.Error("byteBuf:", byteBuf.Buf.String())
		t.Error("strBuf:", strBuf.Buf.String())
	}
}

func TestCron(t *testing.T) {
	ps, _ := NewPsLog()
	defer ps.Close()

	strBuf := new(StrBuf)
	byteBuf := new(BytesBuf)
	stdout := new(Stdout)
	tmp := tmpDir + "/test2cron.log"
	handler := &Handler{
		CleanOffset: true,
		Change:      -1, // 每次都持久化 offset
		// Tail:     true,     // 实时监听
		ExpireAt: NoExpire, // 文件句柄不过期
		Targets: []*Target{
			{
				Content:  " ",
				Excludes: []string{},
				To:       []PsLogWriter{strBuf, byteBuf, stdout},
			},
		},
	}
	if err := ps.Register(handler); err != nil {
		t.Fatal(err)
	}
	if err := ps.AddPaths(tmp); err != nil {
		t.Fatal(err)
	}

	closeCh := make(chan struct{})
	go func() {
		fh := xfile.NewFileHandle(tmp)
		if err := fh.Initf(os.O_WRONLY | os.O_TRUNC); err != nil {
			plg.Error(err)
			return
		}
		defer fh.Close()

		f := fh.GetFile()
		for i := 0; i < 10; i++ {
			// time.Sleep(time.Microsecond)
			_, err := f.WriteString(time.Now().Format(base.DatetimeFmt+".000") + " " + fmt.Sprint(i) + "\n")
			if err != nil {
				plg.Error("write err:", err)
			}
		}
		time.Sleep(time.Second * 5)
		close(closeCh)
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ps.CronLogs()
		case <-closeCh:
			goto stopFor
		}
	}

stopFor:
	data, err := xfile.GetContent(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if byteBuf.Buf.String() != strBuf.Buf.String() && byteBuf.Buf.String() != data {
		t.Error("data:", data)
		t.Error("byteBuf:", byteBuf.Buf.String())
		t.Error("strBuf:", strBuf.Buf.String())
	}
}
