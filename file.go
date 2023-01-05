package pslog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gitee.com/xuesongtao/gotool/base"
)

type FileInfo struct {
	closed       bool // 标记是否已关闭
	f            *os.File
	Handler      *Handler // 这里优先 PsLog.handler
	Dir          string   // 文件目录
	Name         string   // 文件名
	offsetChange int32    // 记录 offset 变化次数
	offset       int64    // 当前文件偏移量
}

// GetF 获取文件句柄
func (f *FileInfo) GetF() *os.File {
	return f.f
}

// Close
func (f *FileInfo) Close() error {
	f.closed = true
	return f.f.Close()
}

// ReOpen 重新 os.Open
func (f *FileInfo) ReOpen() error {
	filename := f.FileName()
	if f.f != nil {
		if err := f.Close(); err != nil {
			return fmt.Errorf("f.Close %q is failed, err: %v", filename, err)
		}
		return nil
	}

	var err error
	f.f, err = os.Open(filename)
	if err != nil {
		return fmt.Errorf("os.Open %q is failed, err: %v", filename, err)
	}
	return nil
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
	return filepath.Join(".", f.Dir, offsetDir, "_"+f.CleanNameFmt()+".txt")
}

// initOffset 初始化文件 offset
func (f *FileInfo) initOffset() {
	if f.offset > 0 {
		return
	}

	// 需要判断下是否已处理过
	offset, err := getContent(f.offsetFilename())
	if err != nil {
		logger.Errorf("getContent %q is failed, err: %v", f.offsetFilename(), err)
		return
	}
	offsetInt, _ := strconv.Atoi(offset)
	f.offset = int64(offsetInt)
}

// saveOffset 保存偏移量
// 通过隐藏文件来保存
func (f *FileInfo) saveOffset() {
	_, err := putContent(f.offsetFilename(), base.ToString(f.offset))
	if err != nil {
		logger.Errorf("putContent %q is failed, err: %v", f.offsetFilename(), err)
	}
}

// putContents 向文件里写内容, 只会复写
func putContent(path string, content string) (int, error) {
	fh, err := filePool.Get(path, os.O_WRONLY|os.O_TRUNC|os.O_RDONLY)
	if err != nil {
		return 0, fmt.Errorf("filePool.Get %q is failed, err: %v", path, err)
	}
	defer filePool.Put(fh)

	f := fh.GetFile()
	return f.WriteString(content)
}

// getContent 获取文件内容
func getContent(path string) (string, error) {
	fh, err := filePool.Get(path, os.O_RDONLY|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return "", fmt.Errorf("filePool.Get %q is failed, err: %v", path, err)
	}
	defer filePool.Put(fh)

	data := make([]byte, 0, 100)
	f := fh.GetFile()
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
		n, err := f.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}
	return string(data), nil
}
