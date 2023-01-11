package pslog

import (
	"gitee.com/xuesongtao/gotool/base"
)

type node struct {
	isNull   bool // 用于标记 tire 除根以外是否为空, 只在 root node 记录有效
	IsRoot   bool
	IsEnd    bool
	Data     byte
	// Children [255]*node // TODO 待优化
	Children map[byte]*node // TODO 待优化
	Target   *Target
}

func newNode(b byte, root ...bool) *node {
	isRoot := false
	if len(root) > 0 && root[0] {
		isRoot = root[0]
	}
	obj := &node{
		IsRoot: isRoot,
		Data:   b,
		Children: make(map[byte]*node),
	}

	if isRoot {
		obj.isNull = true // 根默认为 null
	}
	return obj
}

func (n *node) String() string {
	return base.ToString(n)
}

// 字典树
type tire struct {
	root *node
}

func newTire() *tire {
	// 根节点设置为 '/'
	return &tire{root: newNode('/', true)}
}

// insert 新增模式串
func (t *tire) insert(bytes []byte, target ...*Target) {
	if t.null() { // 如果为空的话, 修改下标记
		t.root.isNull = false
	}
	dataLen := len(bytes)
	curNode := t.root
	var b byte
	for i := 0; i < dataLen; i++ {
		b = bytes[i]
		if node := curNode.Children[b]; node == nil {
			curNode.Children[b] = newNode(b)
		}
		curNode = curNode.Children[b]
	}
	curNode.IsEnd = true
	if len(target) > 0 {
		curNode.Target = target[0]
	}
}

// search 查询主串
func (t *tire) search(target []byte) bool {
	node := t.searchNode(target)
	return node.IsEnd
}

// getTarget 获取 target
func (t *tire) getTarget(target []byte) (*Target, bool) {
	node := t.searchNode(target)
	return node.Target, node.IsEnd && node.Target != nil
}

func (t *tire) searchNode(target []byte) *node {
	dataLen := len(target)
	curNode := t.root
	var b byte
	for i := 0; i < dataLen; i++ {
		b = target[i]
		if node := curNode.Children[b]; node == nil {
			if curNode.IsEnd { // 匹配
				break
			}

			if !curNode.IsRoot { // 当没有匹配同时不在顶层, 返回顶层
				curNode = t.root
			}
			continue
		}
		curNode = curNode.Children[b]
	}
	// logger.Info(curNode)
	return curNode
}

// null 是否为空
func (t *tire) null() bool {
	return t.root.isNull
}
