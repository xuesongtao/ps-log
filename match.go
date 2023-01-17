package pslog

import (
	"bytes"

	"gitee.com/xuesongtao/gotool/base"
)

type Matcher interface {
	Null() bool
	Insert(bytes []byte, target ...*Target)
	Search(target []byte) bool
	GetTarget(target []byte) (*Target, bool)
}

// *******************************************************************************
// *                             普通                                            *
// *******************************************************************************

// Simple 简单匹配
type Simple struct {
	target *Target
	match  []byte // 待匹配的内容
}

func (s *Simple) Null() bool {
	return len(s.match) == 0
}

func (s *Simple) Insert(bytes []byte, target ...*Target) {
	s.match = bytes
	if len(target) > 0 {
		s.target = target[0]
	}
}

func (s *Simple) GetTarget(target []byte) (*Target, bool) {
	if bytes.Contains(target, s.match) {
		return s.target, true && s.target != nil
	}
	return nil, false
}

func (s *Simple) Search(target []byte) bool {
	if len(s.match) == 0 {
		return false
	}
	return bytes.Contains(target, s.match)
}

// *******************************************************************************
// *                             字典树                                           *
// *******************************************************************************

// 字典树
type Tire struct {
	root *node
}

type node struct {
	isNull   bool // 用于标记 tire 除根以外是否为空, 只在 root node 记录有效
	IsRoot   bool
	IsEnd    bool
	Data     byte
	Children [255]*node // TODO 待优化
	target   *Target
}

func newNode(b byte, root ...bool) *node {
	isRoot := false
	if len(root) > 0 && root[0] {
		isRoot = root[0]
	}
	obj := &node{
		IsRoot: isRoot,
		Data:   b,
	}

	if isRoot {
		obj.isNull = true // 根默认为 null
	}
	return obj
}

func (n *node) String() string {
	return base.ToString(n)
}

func newTire() *Tire {
	// 根节点设置为 '/'
	return &Tire{root: newNode('/', true)}
}

// null 是否为空
func (t *Tire) Null() bool {
	return t.root.isNull
}

// Insert 新增模式串
func (t *Tire) Insert(bytes []byte, target ...*Target) {
	if t.Null() { // 如果为空的话, 修改下标记
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
		curNode.target = target[0]
	}
}

// Search 查询主串
func (t *Tire) Search(target []byte) bool {
	node := t.searchNode(target)
	return node.IsEnd
}

// GetTarget 获取 target
func (t *Tire) GetTarget(target []byte) (*Target, bool) {
	node := t.searchNode(target)
	return node.target, node.IsEnd && node.target != nil
}

func (t *Tire) searchNode(target []byte) *node {
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
	// plg.Info(curNode)
	return curNode
}
