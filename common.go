package pslog

import (
	"errors"
	"io"
	"time"

	"gitee.com/xuesongtao/gotool/base"
)

const (
	defaultHandleChange = 100
)

var (
	NoExpire = base.Datetime2TimeObj("9999-12-31 23:59:59") // 不过期
)

// Handler 处理的部分
type Handler struct {
	inited       bool        // 标记是否初始化
	Tail         bool        // 是否实时处理, 说明: true 为实时; false 需要外部定时调用
	Change       int32       // 文件 offset 变化次数, 为持久化文件偏移量数阈值, 当, 说明: -1 为实时保存; 0 达到默认值 defaultHandleChange 时保存; 其他 大于后会保存
	ExpireAt     time.Time   // 文件句柄过期时间, 如: 2022-12-03 11:11:10
	Tos          []io.Writer // 外部处理方法
	Targets      []string    // 目标 msg
	targetsTrie  *tire       // tire 树
	Excludes     []string    // 排除 msg
	excludesTrie *tire       // tire 树
}

func (h *Handler) Valid() error {
	if h.ExpireAt.IsZero() {
		return errors.New("ExpireAt is required")
	}

	if len(h.Tos) == 0 {
		return errors.New("Tos is required")
	}

	// if len(h.Targets) == 0 {
	// 	return errors.New("Targets is required")
	// }
	return nil
}

func (h *Handler) init() {
	if h.inited {
		return
	}

	if h.Change == 0 {
		h.Change = defaultHandleChange
	}

	// 预处理 targets, exclude
	h.targetsTrie = newTire()
	for _, target := range h.Targets {
		h.targetsTrie.insert([]byte(target))
	}
	h.excludesTrie = newTire()
	for _, exclude := range h.Excludes {
		h.excludesTrie.insert([]byte(exclude))
	}

	h.inited = true
}
