package pslog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"gitee.com/xuesongtao/ps-log/line"
)

type PsLogWriter interface {
	WriteTo(bus *LogHandlerBus)
}

type Stdout struct{}

func (p *Stdout) WriteTo(bus *LogHandlerBus) {
	os.Stdout.WriteString(bus.Msg)
}

// Target 目标内容
type Target struct {
	no       int    // 自增编号
	Content  string // 目标内容
	excludes Matcher
	Excludes []string      // 排除 msg
	To       []PsLogWriter // 处理
}

// Handler 处理的部分
type Handler struct {
	CleanOffset bool          // 是否需要清理保存的 offset
	Tail        bool          // 是否实时处理, 说明: true 为实时; false 需要外部定时调用
	Change      int32         // 文件 offset 变化次数, 为持久化文件偏移量数阈值, 当, 说明: -1 为实时保存; 0 达到默认值 defaultHandleChange 时保存; 其他 大于后会保存
	ExpireDur   time.Duration // 文件句柄过期间隔, 常用于全局配置
	ExpireAt    time.Time     // 文件句柄过期时间, 优先 ExpireDur 如: 2022-12-03 11:11:10
	MergeLine   line.Merger   // 行合并, 默认 单行处理
	targets     Matcher
	Targets     []*Target // 目标 msg
	Ext         string    // 外部存入, 回调返回
}

// initMatcher 初始化匹配
// arrLen 为匹配的数组长度
func (h *Handler) initMatcher(arrLen int) Matcher {
	if arrLen <= 1 {
		return &Simple{}
	}
	return newTire()
}

func (h *Handler) Valid() error {
	if h.ExpireAt.IsZero() && h.ExpireDur == 0 {
		return errors.New("ExpireAt, ExpireDur can not both null")
	}

	for i, target := range h.Targets {
		if target.Content == "" {
			return fmt.Errorf("Targets.Content[%d] is null", i)
		}
		if target.To == nil {
			return fmt.Errorf("%q[%d] To is null", target.Content, i)
		}
	}

	if len(h.Targets) == 0 {
		return errors.New("Targets is required")
	}
	return nil
}

func (h *Handler) init() {
	if h.Change == 0 {
		h.Change = defaultHandleChange
	}

	if h.ExpireAt.IsZero() {
		h.ExpireAt = time.Now().Add(h.ExpireDur)
	}

	if h.MergeLine == nil {
		h.MergeLine = line.NewSing()
	}

	// 预处理 targets, exclude
	h.targets = h.initMatcher(len(h.Targets))
	no := 1
	for _, target := range h.Targets {
		if target.Content == "" {
			continue
		}
		target.no = no
		h.targets.Insert([]byte(target.Content), target)
		target.excludes = h.initMatcher(len(target.Excludes))
		for _, exclude := range target.Excludes {
			if exclude == "" {
				continue
			}
			target.excludes.Insert([]byte(exclude), nil)
		}
		no++
	}
}

func (h *Handler) getTargetDump() string {
	data := ""
	for _, v := range h.Targets {
		data += "【" + v.Content + "】"
	}
	return data
}

func (h *Handler) getExcludesDump() string {
	data := ""
	for _, t := range h.Targets {
		es := ""
		for _, e := range t.Excludes {
			if es == "" {
				es += e
			} else {
				es += ";" + e
			}
		}
		if es == "" {
			continue
		}
		data += "【" + t.Content + "排除" + es + "】"
	}
	return data
}

// logHandler 解析到的内容
type LogHandlerBus struct {
	LogPath string // log 的路径
	Msg     string // buf 中的 string
	Ext     string // Handler 中的 Ext 值

	buf *bytes.Buffer
	tos []PsLogWriter
}

func (l *LogHandlerBus) skip() bool {
	l.Msg = l.buf.String()
	l.buf.Reset()
	return l.Msg == ""
}

func (l *LogHandlerBus) Write(b []byte) {
	l.buf.WriteString(string(b) + "\n")
}

func (l *LogHandlerBus) Reset() {
	l.LogPath = ""
	l.Msg = ""
	l.buf.Reset()
	l.tos = nil
}
