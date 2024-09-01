package pslog

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gitee.com/xuesongtao/gotool/base"
	plg "gitee.com/xuesongtao/ps-log/log"
	fs "github.com/fsnotify/fsnotify"
)

const (
	saveOffsetDir         = ".pslog" // 保存偏移量的文件目录
	cleanOffsetFileDayDur = 3        // 清理偏移量文件变动多少天之前的文件
)

type FileInfo struct {
	Dir                 string   // 文件目录路径
	Name                string   // 文件名
	Handler             *Handler // 这里优先 PsLog.handler
	mu                  sync.Mutex
	watchChangeFilename string
	filename            string

	// 父级特有参数
	op       fs.Op
	children map[string]*FileInfo // 如果当前为目录的时候, 这里就有值, key: 文件名[这里不是全路径]

	// 子级特有参数
	fh           *os.File // 存放的文件句柄, 只有 可读权限, key: filename
	offsetChange int32    // 记录 offset 变化次数
	offset       int64    // 当前文件偏移量
	beginOffset  int64    // 记录最开始的偏移量
}

// NewFileInfo 初始化
func NewFileInfo(path string, handler *Handler) (*FileInfo, error) {
	fileInfo := &FileInfo{
		Dir:                 "",
		Name:                "",
		Handler:             handler,
		mu:                  sync.Mutex{},
		watchChangeFilename: "",
		op:                  0,
		children:            make(map[string]*FileInfo),
		fh:                  nil,
		offsetChange:        0,
		offset:              0,
		beginOffset:         0,
	}

	if !handler.initd {
		handler.path = path
		if err := handler.init(); err != nil {
			return nil, err
		}
	}
	fileInfo.init()
	return fileInfo, nil
}

func (f *FileInfo) init() error {
	// fmt.Println("-----", f.Handler.path, f.IsDir())
	f.Parse(f.Handler.path)
	if !f.IsDir() {
		f.initFh()
		f.initOffset()
		return nil
	}

	// 如果是目录的话需要初始化 children
	return f.loopDir(f.Handler.path, func(info os.FileInfo) error {
		filename := filepath.Join(f.Dir, info.Name())
		// fmt.Println("=====",filename)
		if f.needCollect(filename) {
			tmp, err := NewFileInfo(filename, f.Handler.copy())
			if err != nil {
				return err
			}
			f.children[info.Name()] = tmp
		}
		return nil
	})
}

func isRename(op fs.Op) bool {
	return op.Has(fs.Rename)
}

func isCreate(op fs.Op) bool {
	return op.Has(fs.Create)
}

func (f *FileInfo) initFh() error {
	if f.fh != nil {
		return nil
	}

	ff, err := os.Open(f.FileName())
	if err != nil {
		return err
	}
	f.fh = ff
	return nil
}

func (f *FileInfo) resetFn() {
	f.closeFileHandle()
	f.offset = 0
	f.saveOffset(true, f.offset)
}

// Extension 延期
func (f *FileInfo) Extension() {
	f.Handler.ExpireAt = time.Now().Add(f.Handler.ExpireDur)
}

// getFileInfo 根据文件全路径名获取 FileInfo
func (f *FileInfo) getFileInfo(filename string) (*FileInfo, error) {
	// fmt.Println("getFileInfo:", filename, f.FileName())
	if !f.IsDir() { // 当前 FileInfo 为文件
		return f, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	// FileInfo 为目录
	basename := filepath.Base(filename)
	tmp, ok := f.children[basename]
	if ok {
		return tmp, nil
	}
	tmp, err := NewFileInfo(filename, f.Handler.copy())
	if err != nil {
		return nil, err
	}
	f.children[basename] = tmp
	return tmp, nil
}

// HandlerIsNil
func (f *FileInfo) HandlerIsNil() bool {
	return f.Handler == nil
}

// getFileHandle 获取文件 handle
func (f *FileInfo) getFileHandle() (*os.File, error) {
	if err := f.initFh(); err != nil {
		return nil, err
	}
	return f.fh, nil
}

// closeFileHandle 检查句柄账号
func (f *FileInfo) closeFileHandle() {
	if f.fh == nil {
		return
	}
	f.fh.Close()
	f.fh = nil
}

// expireClose 到期关闭句柄
func (f *FileInfo) expireClose() {
	if !f.IsExpire() {
		return
	}

	if !f.IsDir() {
		f.closeFileHandle()
		return
	}

	for _, fileInfo := range f.children {
		fileInfo.expireClose()
	}
}

// NeedCollect 判断下是否需要被采集
func (f *FileInfo) needCollect(filename string) bool {
	if f.HandlerIsNil() {
		return false
	}
	return f.Handler.NeedCollect(filename)
}

// IsDir 是否为目录
func (f *FileInfo) IsDir() bool {
	return f.Handler.isDir
}

// FileName 获取文件的全路径名
func (f *FileInfo) FileName() string {
	if f.filename != "" {
		return f.filename
	}
	f.filename = filepath.Join(f.Dir, f.Name)
	return f.filename
}

// Parse 解析 path
func (f *FileInfo) Parse(path string) {
	if f.IsDir() {
		f.Dir = path
		return
	}
	f.Dir, f.Name = filepath.Split(path)
}

// IsExpire 文件是否过期
func (f *FileInfo) IsExpire(t ...time.Time) bool {
	defaultTime := time.Now()
	if len(t) > 0 {
		defaultTime = t[0]
	}
	if !f.HandlerIsNil() {
		return f.Handler.ExpireAt.Before(defaultTime)
	}
	return false
}

// CleanNameFmt 清理 Name 中的格式, 如: test.log => test
func (f *FileInfo) CleanNameFmt() string {
	if f.Name == "" {
		return ""
	}
	tmpIndex := strings.Index(f.Name, ".")
	return f.Name[:tmpIndex]
}

// PsLogDir 目录
func (f *FileInfo) PsLogDir() string {
	return filepath.Join(f.Dir, saveOffsetDir)
}

// offsetDir 保存偏移量文件的目录
func (f *FileInfo) offsetDir() string {
	return filepath.Join(f.PsLogDir(), "offset")
}

// offsetFilename 获取保存文件偏移量的名称
func (f *FileInfo) offsetFilename(suffix ...string) string {
	// 处理为 xxx/.pslog/offset/_xxx.log.txt
	name := f.Name
	if len(suffix) > 0 && suffix[0] != "" {
		name += "_" + suffix[0]
	}
	return filepath.Join(f.offsetDir(), "_"+name+".txt")
}

func (f *FileInfo) storeOffset(o int64) {
	atomic.StoreInt64(&f.offset, o)
}

func (f *FileInfo) loadOffset() int64 {
	return atomic.LoadInt64(&f.offset)
}

func (f *FileInfo) loadBeginOffset() int64 {
	return atomic.LoadInt64(&f.beginOffset)
}

// initOffset 初始化文件 offset
func (f *FileInfo) initOffset() {
	// 清理已过期文件偏移量文件
	f.removeOffsetFile()

	// 初次使用需要判断下是否需要清除偏移量
	if f.cleanOffset() {
		return
	}

	// 需要判断下是否已处理过
	if f.offset > 0 {
		return
	}

	// 从文件中读取偏移量
	filename := f.offsetFilename()
	offset, err := f.getContent(filename)
	if err != nil {
		plg.Errorf("f.getContent %q is failed, err: %v", filename, err)
		return
	}
	offsetInt, _ := strconv.Atoi(offset)
	f.offset = int64(offsetInt)
	f.beginOffset = f.offset
}

func (f *FileInfo) cleanOffset() (skip bool) {
	if !f.Handler.CleanOffset {
		return
	}
	f.offset = 0
	f.beginOffset = f.offset
	f.putContent(f.offsetFilename(), "0")
	f.Handler.CleanOffset = false
	skip = true
	return
}

// saveOffset 保存偏移量
// 通过隐藏文件来保存
func (f *FileInfo) saveOffset(mustSaveOffset bool, offset int64) {
	filename := f.offsetFilename()
	// 判断下是否需要持久化
	if mustSaveOffset || f.Handler.Change == -1 {
		if _, err := f.putContent(filename, base.ToString(offset)); err != nil {
			plg.Error("f.putContent is failed, err:", err)
		}
		return
	}

	f.offsetChange++
	if f.offsetChange > f.Handler.Change {
		if _, err := f.putContent(filename, base.ToString(offset)); err != nil {
			plg.Error("f.putContent is failed, err:", err)
		}
		f.removeOffsetFile()
		f.offsetChange = 0
	}
}

// getContent 查询
func (f *FileInfo) getContent(path string) (string, error) {
	fh, err := filePool.Get(path, os.O_RDWR)
	if err != nil {
		return "", fmt.Errorf("filePool.Get %q is failed, err: %v", path, err)
	}
	defer filePool.Put(fh)

	data, err := fh.GetContent()
	if err != nil {
		return "", fmt.Errorf("getContent %q is failed, err: %v", path, err)
	}
	return data, nil
}

// putContent 覆写
func (f *FileInfo) putContent(path string, content string) (int, error) {
	fh, err := filePool.Get(path, os.O_RDWR|os.O_TRUNC)
	if err != nil {
		return 0, fmt.Errorf("filePool.Get %q is failed, err: %v", path, err)
	}
	defer filePool.Put(fh)
	return fh.PutContent(content)
}

// remove 移出
func (f *FileInfo) remove(path string) {
	filePool.Remove(path)
}

// removeOffsetFile 移除保存文件偏移量的文件
func (f *FileInfo) removeOffsetFile(filename ...string) {
	if len(filename) > 0 && filename[0] != "" {
		if err := os.Remove(filename[0]); err != nil {
			plg.Errorf("os.Remove %q is failed, err: %v", filename[0], err)
		}
		f.remove(filename[0])
		return
	}

	// 移除当前目录下cleanOffsetFileDayDur天前的文件
	curTime := time.Now()
	f.loopDir(f.offsetDir(), func(info os.FileInfo) error {
		if curTime.Sub(info.ModTime())/base.DayDur <= cleanOffsetFileDayDur {
			return nil
		}

		delFilename := filepath.Join(f.offsetDir(), info.Name())
		if err := os.Remove(delFilename); err != nil {
			plg.Warningf("os.Remove %q is failed, err: %v", delFilename, err)
		}
		f.remove(delFilename)
		return nil
	})
}

// loopDir 遍历目录, 只会遍历一级子级
func (f *FileInfo) loopDir(path string, handle func(info os.FileInfo) error) error {
	entrys, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("ioutil.ReadDir %q is failed, err: %v", path, err)
	}
	for _, entry := range entrys {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := handle(info); err != nil {
			return err
		}
	}
	return nil
}
