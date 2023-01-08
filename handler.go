package pslog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"
)

// Target 目标内容
type Target struct {
	no       int       // 自增编号
	Content  string    // 目标内容
	Excludes []string  // 排除 msg
	excludes *tire     // tire 树
	To       io.Writer // 处理
}

// Handler 处理的部分
type Handler struct {
	Tail     bool      // 是否实时处理, 说明: true 为实时; false 需要外部定时调用
	Change   int32     // 文件 offset 变化次数, 为持久化文件偏移量数阈值, 当, 说明: -1 为实时保存; 0 达到默认值 defaultHandleChange 时保存; 其他 大于后会保存
	ExpireAt time.Time // 文件句柄过期时间, 如: 2022-12-03 11:11:10
	targets  *tire     // tire 树
	Targets  []*Target // 目标 msg
}

func (h *Handler) Valid() error {
	if h.ExpireAt.IsZero() {
		return errors.New("ExpireAt is required")
	}

	for _, target := range h.Targets {
		if target.Content != "" && target.To == nil {
			return fmt.Errorf("%q To is null", target.Content)
		}
	}

	// if len(h.Targets) == 0 {
	// 	return errors.New("Targets is required")
	// }
	return nil
}

func (h *Handler) init() {
	if h.Change == 0 {
		h.Change = defaultHandleChange
	}

	// 预处理 targets, exclude
	h.targets = newTire()
	no := 1
	for _, target := range h.Targets {
		target.no = no
		h.targets.insert([]byte(target.Content), target)
		target.excludes = newTire()
		for _, exclude := range target.Excludes {
			target.excludes.insert([]byte(exclude), nil)
		}
		no++
	}
}

// logHandler 解析到的内容
type logHandler struct {
	msg *bytes.Buffer
	w   io.Writer
}

func newLogHandler(w io.Writer) *logHandler {
	return &logHandler{msg: new(bytes.Buffer), w: w}
}
