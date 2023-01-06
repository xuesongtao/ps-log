package pslog

import (
	"gitee.com/xuesongtao/gotool/base"
)

type node struct {
	IsRoot      bool
	IsEnd       bool
	Data        byte
	ChildrenMap [128]*node
}

func newNode(b byte, root ...bool) *node {
	isRoot := false
	if len(root) > 0 && root[0] {
		isRoot = root[0]
	}
	return &node{
		IsRoot: isRoot,
		Data:   b,
	}
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
func (t *tire) insert(bytes []byte) {
	dataLen := len(bytes)
	curNode := t.root
	for i := 0; i < dataLen; i++ {
		b := bytes[i]
		if node := curNode.ChildrenMap[b]; node == nil {
			curNode.ChildrenMap[b] = newNode(b)
		}
		curNode = curNode.ChildrenMap[b]
	}
	curNode.IsEnd = true
}

// search 查询主串
func (t *tire) search(target []byte) bool {
	dataLen := len(target)
	curNode := t.root
	for i := 0; i < dataLen; i++ {
		b := target[i]
		if node := curNode.ChildrenMap[b]; node == nil {
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
	// fmt.Println(curNode)
	return curNode.IsEnd
}
