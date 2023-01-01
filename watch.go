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
	fileMap  map[string]*WatchFileInfo // key: file path
	filePool *xfile.FilePool           // 缓存所有涉及到的 文件句柄
	watcher  *fsnotify.Watcher         // 监听
}

// WatchFileInfo
type WatchFileInfo struct {
	Dir           bool   // 是否为目录
	Path          string // 文件路径, 这里可能是文件路径或目录路径
	WatchFilePath string // 监听到的变化的文件路径
}

// NewTailFiles
// filePoolSize 控制缓存待监听文件句柄的 pool 大小
func NewWatch(filePoolSize int) (*Watch, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify.NewWatcher is failed, err:%v", err)
	}

	obj := &Watch{
		fileMap:  make(map[string]*WatchFileInfo),
		filePool: xfile.NewFilePool(filePoolSize),
		watcher:  watcher,
	}
	return obj, nil
}

// Add 添加待 watch 的路径
// 说明:
//	1. paths 中可以报目录和文件
// 	2. 建议使用绝对路径
//  3. 自行去重
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
		watchFileInfo := &WatchFileInfo{Dir: false, Path: path}
		if st.IsDir() {
			watchFileInfo.Dir = true
		}
		w.fileMap[path] = watchFileInfo
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
func (w *Watch) Watch(busCh chan *WatchFileInfo) {
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
			watchFileInfo := w.getWatchFileInfo(event.Name)
			if watchFileInfo == nil {
				continue
			}
			busCh <- watchFileInfo
		}
	}
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

func (w *Watch) inEvenOps(target fsnotify.Op, ins ...fsnotify.Op) bool {
	for _, in := range ins {
		if in == target {
			return true
		}
	}
	return false
}
