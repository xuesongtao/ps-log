package pslog

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"gitee.com/xuesongtao/gotool/xfile"
	"github.com/fsnotify/fsnotify"
)

// Watch 监听的文件
type Watch struct {
	fileMap  map[string]*FileInfo // key: file path
	filePool *xfile.FilePool      // 缓存所有涉及到的 文件句柄
	watcher  *fsnotify.Watcher    // 监听
}

// WatchBus 传递参数
type WatchBus struct {
	FileInfo *FileInfo
	FilePool *xfile.FilePool
}

// NewTailFiles
// filePoolSize 控制缓存待监听文件句柄的 pool 大小
func NewWatch(filePoolSize int) (*Watch, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify.NewWatcher is failed, err:%v", err)
	}

	obj := &Watch{
		fileMap:  make(map[string]*FileInfo),
		filePool: xfile.NewFilePool(filePoolSize),
		watcher:  watcher,
	}
	return obj, nil
}

// Add 添加待 watch 的路径
// 说明:
//	1. paths 中可以报目录和文件
// 	2. 建议使用绝对路径
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
		fileInfo := &FileInfo{IsDir: false, Path: path}
		if st.IsDir() {
			fileInfo.IsDir = true
		}
		w.fileMap[path] = fileInfo
		if err := w.watcher.Add(path); err != nil {
			return fmt.Errorf("add %q is failed, err: %v", path, err)
		}
	}
	return nil
}

// Remove 移除待 watch 的路径
func (w *Watch) Remove(files ...string) error {
	for _, file := range files {
		file = filepath.Clean(file)
		delete(w.fileMap, file)
		if err := w.watcher.Remove(file); err != nil {
			return fmt.Errorf("remove %q is failed, err: %v", file, err)
		}
	}
	return nil
}

// Close
func (w *Watch) Close() {
	w.fileMap = nil
	w.filePool = nil
	w.watcher.Close()
}

// Watch 文件监听
func (w *Watch) Watch(handleFn func(*WatchBus)) {
	defer func() {
		if err := recover(); err != nil {
			plog.Error("recover err:", debug.Stack())
		}
		w.Close()
	}()

	for {
		select {
		case err, ok := <-w.watcher.Errors:
			if !ok {
				plog.Info("err channel is closed")
				return
			}
			plog.Error("watch err:", err)
		case event, ok := <-w.watcher.Events:
			if !ok {
				plog.Info("event channel is closed")
				return
			}

			// 只处理 create, write
			if !w.inEvenOps(event.Op, fsnotify.Write, fsnotify.Create) {
				continue
			}

			plog.Infof("filename: %q, op: %s", event.Name, event.Op.String())
			fileInfo := w.getFileInfo(event.Name)
			if fileInfo == nil {
				continue
			}
			handleFn(&WatchBus{
				FileInfo: fileInfo,
				FilePool: w.filePool,
			})
		}
	}
}

func (w *Watch) getFileInfo(filename string) *FileInfo {
	var (
		ok       bool
		fileInfo *FileInfo
	)
	// 这里查找2次, filename 为文件全路径
	// 如果根据 filename 没有查询到, 再按照 filename 目录查询下
	for i := 0; i < 2; i++ {
		fileInfo, ok = w.fileMap[filename]
		if !ok {
			filename = filepath.Dir(filename)
			continue
		}
		break
	}
	return fileInfo
}

func (w *Watch) inEvenOps(target fsnotify.Op, ins ...fsnotify.Op) bool {
	for _, in := range ins {
		if in == target {
			return true
		}
	}
	return false
}
