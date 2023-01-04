package pslog

import (
	"gitee.com/xuesongtao/gotool/base"
)

type Node struct {
	IsRoot      bool
	IsEnd       bool
	Data        byte
	ChildrenMap map[byte]*Node
}

func NewNode(b byte, root ...bool) *Node {
	isRoot := false
	if len(root) > 0 && root[0] {
		isRoot = root[0]
	}
	return &Node{
		IsRoot:      isRoot,
		Data:        b,
		ChildrenMap: make(map[byte]*Node),
	}
}

func (n *Node) String() string {
	return base.ToString(n)
}

// 字典树
type tireTree struct {
	root *Node
}

func newTire() *tireTree {
	// 根节点设置为 '/'
	return &tireTree{root: NewNode('/', true)}
}

// insert 新增模式串
func (t *tireTree) insert(bytes []byte) {
	dataLen := len(bytes)
	curNode := t.root
	for i := 0; i < dataLen; i++ {
		b := bytes[i]
		if _, ok := curNode.ChildrenMap[b]; !ok {
			curNode.ChildrenMap[b] = NewNode(b)
		}
		curNode = curNode.ChildrenMap[b]
	}
	curNode.IsEnd = true
}

// search 查询主串
func (t *tireTree) search(target []byte) bool {
	dataLen := len(target)
	curNode := t.root
	for i := 0; i < dataLen; i++ {
		b := target[i]
		if _, ok := curNode.ChildrenMap[b]; !ok {
			if curNode.IsEnd { // 匹配
				break
			}

			if !curNode.IsRoot { // 当没有匹配同时不在顶层, 返回顶层
				curNode = t.root
			}
			continue
		}
		curNode = curNode.ChildrenMap[b]
	}
	return curNode.IsEnd
}
