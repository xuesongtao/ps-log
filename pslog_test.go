package pslog

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gitee.com/xuesongtao/gotool/base"
	"gitee.com/xuesongtao/gotool/xfile"
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

func TestTail(t *testing.T) {
	ps, _ := NewPsLog(WithPreCleanOffset())
	defer ps.Close()
	ps.TailLogs()

	strBuf := new(StrBuf)
	byteBuf := new(BytesBuf)
	stdout := new(Stdout)
	tmp := tmpDir + "/test2tail.log"
	handler := &Handler{
		Change:   -1,       // 每次都持久化 offset
		Tail:     true,     // 实时监听
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
		time.Sleep(time.Second * 2)
		close(closeCh)
	}()
	if err := ps.AddPaths(tmp); err != nil {
		t.Fatal(err)
	}

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
	ps, _ := NewPsLog(WithPreCleanOffset())
	defer ps.Close()

	strBuf := new(StrBuf)
	byteBuf := new(BytesBuf)
	stdout := new(Stdout)
	tmp := tmpDir + "/test2cron.log"
	handler := &Handler{
		Change: -1, // 每次都持久化 offset
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
