package pslog

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gitee.com/xuesongtao/gotool/base"
)

const (
	saveOffsetDir = "/.pslog_offset" // 保存偏移量的文件目录
)

type FileInfo struct {
	Handler      *Handler // 这里优先 PsLog.handler
	Dir          string   // 文件目录
	Name         string   // 文件名
	offsetChange int32    // 记录 offset 变化次数
	offset       int64    // 当前文件偏移量
}

// FileName 获取文件的全路径名
func (f *FileInfo) FileName() string {
	return filepath.Join(f.Dir, f.Name)
}

// Parse 解析 path
func (f *FileInfo) Parse(path string) {
	f.Dir, f.Name = filepath.Split(path)
}

// IsExpire 文件是否过期
func (f *FileInfo) IsExpire(t ...time.Time) bool {
	defaultTime := time.Now()
	if len(t) > 0 {
		defaultTime = t[0]
	}
	if f.Handler != nil {
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

// setOffset 保存 offset
func (f *FileInfo) setOffset(offset int64) {
	f.offset = offset

	// 判断下是否需要持久化
	if f.Handler.Change == -1 {
		f.saveOffset()
		return
	}

	f.offsetChange++
	if f.offsetChange > f.Handler.Change {
		f.saveOffset()
		f.offsetChange = 0
	}
}

// offsetFilename 获取保存文件偏移量的名称
func (f *FileInfo) offsetFilename() string {
	// 处理为 xxx/.pslog_offset/_xxx.txt
	return filepath.Join(".", f.Dir, saveOffsetDir, "_"+f.CleanNameFmt()+".txt")
}

// initOffset 初始化文件 offset
func (f *FileInfo) initOffset() {
	if f.offset > 0 {
		return
	}

	// 需要判断下是否已处理过
	filename := f.offsetFilename()
	offset, err := f.getContent(filename)
	if err != nil {
		logger.Errorf("f.getContent %q is failed, err: %v", filename, err)
		return
	}
	offsetInt, _ := strconv.Atoi(offset)
	f.offset = int64(offsetInt)
}

// saveOffset 保存偏移量
// 通过隐藏文件来保存
func (f *FileInfo) saveOffset() {
	filename := f.offsetFilename()
	if _, err := f.putContent(filename, base.ToString(f.offset)); err != nil {
		logger.Error("f.putContent is failed, err:", err)
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
