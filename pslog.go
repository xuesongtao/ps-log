package pslog

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"

	tl "gitee.com/xuesongtao/taskpool"
)

var (
	noHandlerErr = errors.New("handler is null, you can call Register first")
)

// Opt
type Opt func(*PsLog)

// WithAsync2Tos 异步处理 tos
func WithAsync2Tos() Opt {
	return func(pl *PsLog) {
		pl.async2Tos = true
	}
}

// WithTaskPoolSize 设置协程池大小
func WithTaskPoolSize(size int) Opt {
	return func(pl *PsLog) {
		if pl.taskPool != nil {
			pl.taskPool.Close()
		}
		pl.taskPool = tl.NewTaskPool("parse log", size)
	}
}

// PsLog 解析 log
type PsLog struct {
	tail      bool // 是否需要实时分析
	async2Tos bool // 是否异步处理 tos
	closed    bool
	rwMu      sync.RWMutex
	taskPool  *tl.TaskPool         // 任务池
	handler   *Handler             // 处理部分
	watch     *Watch               // 文件监听
	watchCh   chan *WatchFileInfo  // 文件监听文件内容
	logMap    map[string]*FileInfo // 需要处理的 log, 不需要手动是否句柄, 通过 lru 进行淘汰, key: 文件路径
}

// NewPsLog 是根据提供的 log path 进行逐行解析
// 注: 结束时需要调用 Close
func NewPsLog(opts ...Opt) (*PsLog, error) {
	obj := &PsLog{
		logMap:   make(map[string]*FileInfo),
		handler:  new(Handler),
		taskPool: tl.NewTaskPool("parse log", runtime.NumCPU()),
	}

	for _, opt := range opts {
		opt(obj)
	}
	return obj, nil
}

// Register 注册处理器
func (p *PsLog) Register(handler *Handler) error {
	p.handler = handler
	return nil
}

// AddPaths 添加 path, path 必须为文件全路径
// 根据 PsLog.Handler 进行处理
func (p *PsLog) AddPaths(paths ...string) error {
	if p.handler == nil {
		return noHandlerErr
	}
	path2HandlerMap := make(map[string]*Handler, len(paths))
	for _, path := range paths {
		path2HandlerMap[path] = p.handler
	}
	return p.addLogPath(path2HandlerMap)
}

// AddPath2HandlerMap 添加文件对应的处理方法
// 只会根据文件对应的 Handler 进行处理
func (p *PsLog) AddPath2HandlerMap(path2HandlerMap map[string]*Handler) error {
	return p.addLogPath(path2HandlerMap)
}

// addLogPath 添加 log path, 同时添加监听 log path
func (p *PsLog) addLogPath(path2HandlerMap map[string]*Handler) error {
	p.rwMu.Lock()
	defer p.rwMu.Unlock()

	for path, handler := range path2HandlerMap {
		path = filepath.Clean(path)
		if _, ok := p.logMap[path]; ok {
			continue
		}

		// 处理 handler
		if handler == nil {
			return fmt.Errorf("%q no has handler", path)
		}
		if err := handler.Valid(); err != nil {
			return fmt.Errorf("%q handler is not ok, err: %v", path, err)
		}
		handler.init()

		// 判断下是否为目录
		st, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("os.Stat %q is failed, err: %v", path, err)
		}
		if st.IsDir() {
			return fmt.Errorf("%q is dir, it should file", path)
		}

		// 保存 file
		fileInfo := &FileInfo{Handler: handler}
		fileInfo.Parse(path)
		fileInfo.initOffset()
		p.logMap[path] = fileInfo
		if p.tail && handler.Tail {
			if err := p.watch.Add(path); err != nil {
				return fmt.Errorf("p.watch.Add is failed, err: %v", err)
			}
		}
	}
	return nil
}

// Close 释放资源
func (p *PsLog) Close() {
	p.rwMu.Lock()
	defer p.rwMu.Unlock()

	if p.closed { // 已经关了的就退出, 防止重复关闭 chan panic
		return
	}

	if p.watch != nil {
		p.watch.Close()
	}

	filePool.Close()

	if p.taskPool != nil {
		p.taskPool.SafeClose()
	}

	// close(p.watchCh) // p.watch.Close() 执行后, p.watchCh 会被关闭
	p.closed = true
}

// TailLogs 实时解析 log
// watchSize 为监听到文件变化处理数据的 chan 的长度, 建议为监听文件的个数
func (p *PsLog) TailLogs(watchChSize ...int) error {
	p.tail = true

	size := 1 << 4
	if len(watchChSize) > 0 && watchChSize[0] > 0 {
		size = watchChSize[0]
	}

	// 初始化 watch
	watch, err := NewWatch()
	if err != nil {
		return fmt.Errorf("NewWatch is failed, err:%v", err)
	}
	p.watch = watch
	p.watchCh = make(chan *WatchFileInfo, size)
	p.watch.Watch(p.watchCh)

	// 开始监听
	go func() {
		defer p.final()

		// 退出情况
		// 1. watch 退出
		// 2. Close 后
		for watchInfo := range p.watchCh {
			p.rwMu.RLock()
			fileInfo, ok := p.logMap[watchInfo.Path]
			p.rwMu.RUnlock()
			if !ok {
				logger.Infof("%q is not exist", watchInfo.Path)
				continue
			}
			if !fileInfo.Handler.Tail {
				logger.Infof("%q no need tail", watchInfo.Path)
				continue
			}
			p.parseLog(fileInfo) // 防止在解析的时候, fileInfo 变化
		}
		logger.Info("watchCh is closed")
	}()
	return nil
}

// cronLog 定时解析 log
func (p *PsLog) CronLogs() {

}

// parseLog 解析文件
func (p *PsLog) parseLog(fileInfo *FileInfo) {
	fh, err := filePool.Get(fileInfo.FileName(), os.O_RDONLY)
	if err != nil {
		logger.Errorf("filePool.Get %q is failed, err: %v", fileInfo.FileName(), err)
		return
	}
	defer filePool.Put(fh)

	f := fh.GetFile()
	st, err := f.Stat()
	if err != nil {
		logger.Error("f.Stat %q is failed, err: %v", fileInfo.FileName(), err)
		return
	}

	fileSize := st.Size()
	// logger.Infof("filename: %q, offset: %d, size: %d", fileInfo.FileName(), fileInfo.offset, fileSize)
	if fileSize == 0 || fileInfo.offset > fileSize {
		return
	}

	_, err = f.Seek(fileInfo.offset, io.SeekStart)
	if err != nil {
		logger.Error("f.Seek is failed, err:", err)
		return
	}

	// 逐行读取
	rows := bufio.NewScanner(f)
	readSize := fileInfo.offset
	dataMap := make(map[int]*logHandler, 1<<6) // key: target.no
	for rows.Scan() {
		// 保证本次读取内容小于 fileSize
		if readSize > fileSize {
			break
		}
		data := rows.Bytes()
		readSize += int64(len(data))
		targe, ok := p.parse(fileInfo.Handler, data)
		if !ok {
			continue
		}
		if _, ok := dataMap[targe.no]; !ok {
			dataMap[targe.no] = newLogHandler(targe.To)
		}
		dataMap[targe.no].msg.WriteString(string(data) + "\n")
	}
	p.writer(dataMap)

	// 保存偏移量
	fileInfo.offset = fileSize
	p.taskPool.Submit(func() {
		fileInfo.saveOffset(fileSize)
	})
}

// parse 需要处理
func (p *PsLog) parse(h *Handler, row []byte) (*Target, bool) {
	if h.targets.null() {
		return nil, false
	}
	target, ok := h.targets.getTarget(row)
	if !ok {
		return nil, false
	}
	return target, !target.excludes.search(row)
}

// writer 写入目标, 默认同步处理
func (p *PsLog) writer(dataMap map[int]*logHandler) {
	// 异步
	if p.async2Tos {
		for _, data := range dataMap {
			p.taskPool.Submit(func() {
				data.w.Write(data.msg.Bytes())
			})
		}
		return
	}

	// 同步
	for _, data := range dataMap {
		data.w.Write(data.msg.Bytes())
	}
}

func (p *PsLog) final() {
	if err := recover(); err != nil {
		logger.Errorf("recover err: %v, stack: %s", err, debug.Stack())
	}
	p.Close()
}
