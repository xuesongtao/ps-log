package line

import (
	"bytes"
	"regexp"

	plg "gitee.com/xuesongtao/ps-log/log"
)

// Multi 多行处理
type Multi struct {
	re   *regexp.Regexp
	line []byte
	buf  bytes.Buffer
}

func NewMulti() *Multi {
	return &Multi{}
}

// StartPattern 行开始的正则表达式
func (m *Multi) StartPattern(expr string) error {
	re, err := regexp.Compile(expr)
	if err != nil {
		return err
	}
	m.re = re
	return nil
}

// Line 获取行内容
// 注: 获取完后, 应该调用一次 Residue 获取剩余的内容
func (m *Multi) Line() []byte {
	tmp := m.line
	m.line = nil
	return tmp
}

func (m *Multi) Residue() []byte {
	defer m.buf.Reset()
	return m.buf.Bytes()
}

func (m *Multi) Append(data []byte) bool {
	// 说明:
	// 1. 第一次匹配先清理 buf, 然后追加
	// 2. 第二次匹配就应该上一行的内容
	if m.re.Match(data) {
		m.line = m.buf.Bytes()
		m.buf.Reset()
	}
	if _, err := m.buf.Write(data); err != nil {
		plg.Error("m.data.Write is failed, err:", err)
	}
	return len(m.line) > 0
}
