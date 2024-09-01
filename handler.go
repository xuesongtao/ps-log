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
	To       []PsLogWriter // 一个目标内容, 多种处理方式
	Ext      string        // 外部存入, 回调返回
}

// Handler 处理的部分
type Handler struct {
	LoopParse   bool          // 循环解析, 用于监听单文件日志, 说明: 这个采集的有可能不准确(在这种是基于文件大小和内存记录的偏移量做比较, 模式建议用 tail, cron 的话如果间隔时间太长就可能漏)
	CleanOffset bool          // 是否需要清理保存的 offset, 只限于开机后一次
	Tail        bool          // 是否实时处理, 说明: true 为实时; false 需要外部定时调用
	Change      int32         // 文件 offset 变化次数, 为持久化文件偏移量数阈值, 当, 说明: -1 为实时保存; 0 达到默认值 defaultHandleChange 时保存; 其他 大于后会保存
	ExpireDur   time.Duration // 文件句柄过期间隔, 常用于全局配置, 如果没有, 默认 1 小时
	ExpireAt    time.Time     // 文件句柄过期时间, 优先 ExpireDur 如: 2022-12-03 11:11:10
	MergeRule   line.Merger   // 日志文件行合并规则, 默认 单行处理
	targets     Matcher
	Targets     []*Target                  // 目标 msg
	Ext         string                     // 外部存入, 回调返回
	NeedCollect func(filename string) bool // 当监听的对象为目录时, 判断文件是否需要采集, 注: 采集的 path 为 dir 的时候, 这里必须填

	isDir bool
	path  string // 原始 path
	initd bool   // 是否已经初始化
}

func (h *Handler) copy() *Handler {
	return &Handler{
		LoopParse:   h.LoopParse,
		CleanOffset: h.CleanOffset,
		Tail:        h.Tail,
		Change:      h.Change,
		ExpireDur:   h.ExpireDur,
		ExpireAt:    h.ExpireAt,
		MergeRule:   h.MergeRule,
		// targets:     nil,
		Targets:     h.Targets,
		Ext:         h.Ext,
		NeedCollect: h.NeedCollect,
		// isDir:       false,
		// path:        "",
		// initd:       false,
	}
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

	if len(h.Targets) == 0 {
		return errors.New("Targets is required")
	}

	for i, target := range h.Targets {
		if target.Content == "" {
			return fmt.Errorf("Targets.Content[%d] is null", i)
		}
		if target.To == nil {
			return fmt.Errorf("%q[%d] To is null", target.Content, i)
		}
	}
	return nil
}

func (h *Handler) init() error {
	if h.initd {
		return nil
	}

	if err := h.Valid(); err != nil {
		return err
	}

	h.initd = true
	if h.Change == 0 {
		h.Change = defaultHandleChange
	}

	if h.ExpireDur == 0 {
		h.ExpireDur = time.Hour
	}

	if h.ExpireAt.IsZero() {
		h.ExpireAt = time.Now().Add(h.ExpireDur)
	}

	if h.MergeRule == nil {
		h.MergeRule = line.NewSing()
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

	// 判断下是否为目录
	st, err := os.Stat(h.path)
	if err != nil {
		return fmt.Errorf("os.Stat %q is failed, err: %v", h.path, err)
	}
	h.isDir = st.IsDir()

	if h.isDir && h.NeedCollect == nil {
		return fmt.Errorf("%q is dir, NeedCollect is nil", h.path)
	}
	return nil
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
	LogPath   string // log 的路径
	Msg       string // buf 中的 string
	Ext       string // Handler 中的 Ext 值
	TargetExt string // Target 中的 Ext 值

	buf *bytes.Buffer
	tos []PsLogWriter
}

func (l *LogHandlerBus) skip() bool {
	if l.buf.Len() == 0 {
		return true
	}
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
