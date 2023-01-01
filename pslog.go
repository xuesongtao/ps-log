package pslog

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"gitee.com/xuesongtao/gotool/xfile"
	"gitee.com/xuesongtao/taskpool"
	"gitee.com/xuesongtao/xlog"
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
	tail       bool // 是否实时
	taskPool   *taskpool.TaskPool
	filePool   *xfile.FilePool
	watch      *Watch
	watchCh    chan *WatchFileInfo
	logPathMap map[string]*FileInfo // 需要解析的 log
	tos        []Toer               // 外部处理方法
	targets    []string             // 目标 msg
	excludes   []string             // 排除 msg
}

type FileInfo struct {
	FileName string // 文件名
	Offset   int64  // 当前文件偏移量
}

// NewPsLog 是根据提供的 log path 进行逐行解析
// 注: 结束时需要调用 Close
func NewPsLog(opts ...Opt) (*PsLog, error) {
	obj := &PsLog{
		targets:    make([]string, 0, 5),
		logPathMap: make(map[string]*FileInfo),
	}

	for _, o := range opts {
		o(obj)
	}

	if obj.tail {
		size := 2 << 4
		watch, err := NewWatch(size)
		if err != nil {
			panic(fmt.Sprintf("NewWatch is failed, err:%v", err))
		}
		obj.watch = watch
		obj.watchCh = make(chan *WatchFileInfo, size)
		go obj.watch.Watch(obj.watchCh)
	}
	return obj, nil
}

// Register 注册处理器
func (p *PsLog) Register(h []Toer) {
	p.tos = append(p.tos, h...)
}

// AddLogPaths 添加 log path
func (p *PsLog) AddLogPaths(paths ...string) error {
	for _, path := range paths {
		if _, ok := p.logPathMap[path]; ok {
			continue
		}

		st, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("os.Stat is failed, err: %v", err)
		}

		if st.IsDir() {
			return fmt.Errorf("path must log path, %q is dir", path)
		}

		p.logPathMap[path] = &FileInfo{}
		if p.tail {
			if err := p.watch.Add(path); err != nil {
				return fmt.Errorf("p.watch.Add is failed, err: %v", err)
			}
		}
	}
	return nil
}

// Targets 设置解析目标
func (p *PsLog) Targets(target ...string) {
	p.targets = append(p.targets, target...)
}

// Excludes 设置排除内容
func (p *PsLog) Excludes(excludes ...string) {
	if p.excludes == nil {
		p.excludes = make([]string, 0, len(excludes))
	}
	p.excludes = append(p.excludes, excludes...)
}

func (p *PsLog) Close() {
	p.taskPool.Close()
	p.filePool.Close()
	close(p.watchCh)
}

// TailLog 实时解析 log
func (p *PsLog) TailLog() {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				plog.Error("recover err:", debug.Stack())
			}
			p.Close()
		}()

		for {
			select {
			case watchInfo, ok := <-p.watchCh:
				if !ok {
					break
				}
				fileInfo, ok := p.logPathMap[watchInfo.Path]
				if !ok {
					continue
				}
				fileInfo.FileName = watchInfo.Path
				p.parseLog(fileInfo)
			default:
			}
		}
	}()
}

// cronLog 定时解析 log
func (p *PsLog) CronLog() {

}

func (p *PsLog) parseLog(fileInfo *FileInfo) {
	if p.filePool == nil {
		p.filePool = xfile.NewFilePool()
	}
	fh, err := p.filePool.Get(fileInfo.FileName, os.O_RDONLY)
	if err != nil {
		plog.Error("p.filePool.Get is failed, err:", err)
		return
	}
	defer p.filePool.Put(fh)

	f := fh.GetFile()
	st, err := f.Stat()
	if err != nil {
		plog.Error("f.Stat is failed, err:", err)
		return
	}
	plog.Infof("filename: %q, offset: %d, size: %d", fileInfo.FileName, fileInfo.Offset, st.Size())
	_, err = f.Seek(fileInfo.Offset, io.SeekStart)
	if err != nil {
		plog.Error("fh.GetFile().Seek is failed, err:", err)
		return
	}

	// rowBuf := new(strings.Builder)

	// 逐行读取
	rows := bufio.NewScanner(f)
	for rows.Scan() {
		rowStr := rows.Text()
		if rowStr == "" {
			continue
		}
		xlog.Info("rowStr:", rowStr)
	}

	if st.Size() == 0 {
		return
	}
	fileInfo.Offset = st.Size()
}

// Writer 写入目标, 默认同步处理
func (p *PsLog) Writer(txt string, async ...bool) {
	// 异步
	if len(async) > 0 && async[0] {
		// 通过懒加载方式进行初始化
		if p.taskPool == nil {
			p.taskPool = taskpool.NewTaskPool("parse log", len(p.tos), taskpool.WithProGoWorker())
		}

		for _, to := range p.tos {
			p.taskPool.Submit(func() {
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
