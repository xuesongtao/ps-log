package pslog

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	fs "github.com/fsnotify/fsnotify"
)

// Watch 监听的文件
type Watch struct {
	fileMap map[string]*WatchFileInfo // key: file path
	watcher *fs.Watcher               // 监听
}

// WatchFileInfo
type WatchFileInfo struct {
	IsDir         bool   // 是否为目录
	Path          string // 原始添加的文件路径, 这里可能是文件路径或目录路径
	WatchFilePath string // 监听到的变化的文件全路径
}

// NewWatch 监听
func NewWatch() (*Watch, error) {
	watcher, err := fs.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fs.NewWatcher is failed, err:%v", err)
	}

	obj := &Watch{
		fileMap: make(map[string]*WatchFileInfo),
		watcher: watcher,
	}
	return obj, nil
}

// Add 添加待 watch 的路径
// 说明:
//  1. 自行去重
//	2. paths 中可以为目录和文件
// 	3. 建议使用绝对路径
func (w *Watch) Add(paths ...string) error {
	for _, path := range paths {
		st, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("os.Lstat is failed, err: %v", err)
		}
		path = filepath.Clean(path)
		if _, ok := w.fileMap[path]; ok {
			continue
		}
		watchFileInfo := &WatchFileInfo{IsDir: false, Path: path}
		if st.IsDir() {
			watchFileInfo.IsDir = true
		}

		// 保存和监听
		w.fileMap[path] = watchFileInfo
		if err := w.watcher.Add(path); err != nil {
			return fmt.Errorf("add %q is failed, err: %v", path, err)
		}
	}
	return nil
}

// Remove 移除待 watch 的路径
func (w *Watch) Remove(paths ...string) error {
	for _, path := range paths {
		path = filepath.Clean(path)
		delete(w.fileMap, path)
		if err := w.watcher.Remove(path); err != nil {
			return fmt.Errorf("remove %q is failed, err: %v", path, err)
		}
	}
	return nil
}

// Close
func (w *Watch) Close() {
	w.fileMap = nil
	w.watcher.Close()
}

// Watch 文件异步监听
func (w *Watch) Watch(busCh chan *WatchFileInfo) {
	go func() {
		defer func() {
			w.watcher.Close()
			close(busCh)
			if err := recover(); err != nil {
				logger.Error("recover err:", debug.Stack())
			}
		}()

		for {
			select {
			case err, ok := <-w.watcher.Errors:
				if !ok {
					logger.Info("err channel is closed")
					return
				}
				logger.Error("watch err:", err)
			case event, ok := <-w.watcher.Events:
				if !ok {
					logger.Info("event channel is closed")
					return
				}

				// 只处理 create, write
				if !w.inEvenOps(event.Op, fs.Write, fs.Create) {
					continue
				}

				logger.Infof("filename: %q, op: %s", event.Name, event.Op.String())
				watchFileInfo := w.getWatchFileInfo(event.Name)
				if watchFileInfo == nil {
					continue
				}
				busCh <- watchFileInfo
			}
		}
	}()
}

func (w *Watch) getWatchFileInfo(filename string) *WatchFileInfo {
	var (
		ok            bool
		watchFileInfo *WatchFileInfo
	)
	// 这里查找2次, filename 为文件全路径
	// 如果根据 filename 没有查询到, 再按照 filename 目录查询下
	for i := 0; i < 2; i++ {
		watchFileInfo, ok = w.fileMap[filename]
		if !ok {
			filename = filepath.Dir(filename)
			continue
		}
		break
	}
	return watchFileInfo
}

func (w *Watch) inEvenOps(target fs.Op, ins ...fs.Op) bool {
	for _, in := range ins {
		if in == target {
			return true
		}
	}
	return false
}

// WatchList 查询监听的所有 path
func (w *Watch) WatchList() string {
	buf := new(strings.Builder)
	defer buf.Reset()

	for path := range w.fileMap {
		buf.WriteString(path + "\n")
	}
	return buf.String()
}
