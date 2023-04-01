package line

type Merger interface {
	Null() bool              // 执行结束后, 需要判断是否为空, 如果不为空, 就再调用 Line
	Line() []byte            // 获取 merge 成功 line, 注: 获取完后, 应该调用一次 Residue 获取剩余的内容
	Append(data []byte) bool // 追加行内容, 如果返回 true 表示满足 merge 成功, 应该调用 Line 获取行内容; 反之未完成
}
