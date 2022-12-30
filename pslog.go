package pslog

import (
	"fmt"

	"gitee.com/xuesongtao/gotool/xfile"
	"gitee.com/xuesongtao/taskpool"
)

// Opt
type Opt func(*PsLog)

// WithTail 是否实时监听 log
func WithTail(is bool) Opt {
	return func(pl *PsLog) {
		pl.tail = is
	}
}

// PsLog 解析 log
type PsLog struct {
	tail     bool // 是否实时
	logPath  string
	pool     *taskpool.TaskPool
	tos      []Toer   // 外部处理方法
	targets  []string // 目标 msg
	excludes []string // 排除 msg
}

// NewPsLog 是根据提供的 log path 进行逐行解析
func NewPsLog(path string, opts ...Opt) (*PsLog, error) {
	if !xfile.Exists(path) {
		return nil, fmt.Errorf("path %q is not exist", path)
	}

	obj := &PsLog{
		logPath: path,
		targets: make([]string, 0, 5),
	}

	for _, o := range opts {
		o(obj)
	}
	return obj, nil
}

// Register 注册处理器
func (p *PsLog) Register(h ...Toer) {
	p.tos = append(p.tos, h...)
}

// Target 设置解析目标
func (p *PsLog) Target(target ...string) {
	p.targets = append(p.targets, target...)
}

// Excludes 设置排除内容
func (p *PsLog) Excludes(excludes ...string) {
	if p.excludes == nil {
		p.excludes = make([]string, 0, len(excludes))
	}
	p.excludes = append(p.excludes, excludes...)
}

// tailLog 实时解析 log
func (p *PsLog) TailLog() error {

	return nil
}

// cronLog 定时解析 log
func (p *PsLog) CronLog() {

}

func (p *PsLog) parseLog(offset int64) {
	// fh, err := p.
}

// Writer 写入目标, 默认同步处理
func (p *PsLog) Writer(txt string, async ...bool) {
	// 异步
	if len(async) > 0 && async[0] {
		// 通过懒加载方式进行初始化
		if p.pool == nil {
			p.pool = taskpool.NewTaskPool("parse log", len(p.tos), taskpool.WithProGoWorker())
		}

		for _, to := range p.tos {
			p.pool.Submit(func() {
				to.WriterTo(txt)
			})
		}
		return
	}

	// 同步
	for _, to := range p.tos {
		to.WriterTo(txt)
	}
}
