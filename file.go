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
)

const (
	saveOffsetDir         = ".pslog" // 保存偏移量的文件目录
	cleanOffsetFileDayDur = 3        // 清理偏移量文件变动多少天之前的文件
)

type FileInfo struct {
	Dir     string   // 文件目录路径
	Name    string   // 文件名
	Handler *Handler // 这里优先 PsLog.handler
	mu      sync.Mutex

	// 父级特有参数
	watchChangedFilename string               // 监听到变化的文件名
	isRename             bool                 // 是否修改文件名
	children             map[string]*FileInfo // 如果当前为目录的时候, 这里就有值, key: 文件名[这里不是全路径]

	// 子级特有参数
	fh           *fileHandleInfo // 存放的文件句柄, 只有 可读权限, key: filename
	offsetChange int32           // 记录 offset 变化次数
	offset       int64           // 当前文件偏移量
	beginOffset  int64           // 记录最开始的偏移量
}

type fileHandleInfo struct {
	f          *os.File
	createTime time.Time
}

// NewFileInfo 初始化
func NewFileInfo(path string, handler Handler) (*FileInfo, error) {
	fileInfo := &FileInfo{
		Dir:                  "",
		Name:                 "",
		Handler:              &handler,
		watchChangedFilename: "",
		isRename:             false,
		fh:                   &fileHandleInfo{},
		offsetChange:         0,
		offset:               0,
		beginOffset:          0,
		mu:                   sync.Mutex{},
		children:             map[string]*FileInfo{},
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

func (f *FileInfo) init() {
	f.Parse(f.Handler.path)
	if !f.IsDir() {
		f.initOffset()
		return
	}

	// 如果是目录的话需要初始化 children
	f.loopDir(f.Handler.path, func(info os.FileInfo) error {
		filename := info.Name()
		if f.needCollect(filename) {
			_, err := f.getFileInfo(filepath.Join(f.Dir, filename))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Extension 延期
func (f *FileInfo) Extension() {
	f.Handler.ExpireAt = time.Now().Add(f.Handler.ExpireDur)
}

// getFileInfo 根据文件全路径名获取 FileInfo
func (f *FileInfo) getFileInfo(filename string) (*FileInfo, error) {
	if filename != f.Dir { // 当前 FileInfo 为文件
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
	tmp, err := NewFileInfo(filename, *f.Handler)
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
	filename := f.FileName()
	var err error
	if f.fh == nil {
		ff := new(fileHandleInfo)
		ff.f, err = os.Open(filename)
		if err != nil {
			return nil, err
		}
		ff.createTime = time.Now()
		f.fh = ff
	}
	return f.fh.f, nil
}

// closeFileHandle 检查句柄账号
func (f *FileInfo) closeFileHandle(cleanOffset ...bool) {
	if f.fh == nil {
		return
	}
	f.fh.f.Close()

	if len(cleanOffset) > 0 && cleanOffset[0] {
		f.beginOffset = 0
		f.offset = 0
		f.saveOffset(true, f.offset) // 强刷下
	}
}

// expireClose 到期关闭句柄
func (f *FileInfo) expireClose() {
	if !f.IsExpire(f.fh.createTime) {
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
	return filepath.Join(f.Dir, f.Name)
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
func (f *FileInfo) offsetFilename() string {
	// 处理为 xxx/.pslog/offset/_xxx.log.txt
	return filepath.Join(f.offsetDir(), "_"+f.Name+".txt")
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
