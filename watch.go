package pslog

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"gitee.com/xuesongtao/gotool/base"
	plg "gitee.com/xuesongtao/ps-log/log"
	fs "github.com/fsnotify/fsnotify"
	tw "github.com/olekukonko/tablewriter"
)

// Watch 监听的文件
type Watch struct {
	fileMap map[string]*WatchFileInfo // key: file path
	watcher *fs.Watcher               // 监听
}

// WatchFileInfo
type WatchFileInfo struct {
	IsDir           bool   // 是否为目录
	Dir             string // 原始目录路径
	Path            string // 原始添加的文件路径, 这里可能是文件路径或目录路径
	IsRename        bool   // 是否修改名字
	ChangedFilename string // 被监听到的文件名, 绝对路径
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
//  1. 自动去重
//  2. paths 中可以为目录和文件
//  3. 建议使用绝对路径
func (w *Watch) Add(paths ...string) error {
	for _, path := range paths {
		path = filepath.Clean(path)
		st, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("os.Lstat is failed, err: %v", err)
		}
		if _, ok := w.fileMap[path]; ok {
			continue
		}
		watchFileInfo := &WatchFileInfo{IsDir: false, Path: path, Dir: path}
		if st.IsDir() {
			watchFileInfo.IsDir = true
		} else {
			watchFileInfo.Dir = filepath.Dir(path)
		}

		// 保存和监听
		w.fileMap[path] = watchFileInfo
		// 只监听目录
		if err := w.watcher.Add(watchFileInfo.Dir); err != nil {
			return fmt.Errorf("w.watcher.Add is failed, err: %v", err)
		}
	}
	return nil
}

// Remove 移除待 watch 的路径
func (w *Watch) Remove(paths ...string) error {
	for _, path := range paths {
		path = filepath.Clean(path)
		info, ok := w.fileMap[path]
		if ok && info.IsDir {
			if err := w.watcher.Remove(info.Dir); err != nil {
				return fmt.Errorf("w.watcher.Remove is failed, err: %v", err)
			}
		}
		delete(w.fileMap, path)
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
			if err := recover(); err != nil {
				plg.Error("Watch recover err:", debug.Stack())
			}
			close(busCh)
			w.fileMap = nil
		}()

		for {
			select {
			case err, ok := <-w.watcher.Errors:
				if !ok {
					plg.Info("err channel is closed")
					return
				}
				plg.Error("watch err:", err)
			case event, ok := <-w.watcher.Events:
				if !ok {
					plg.Info("event channel is closed")
					return
				}

				// 只处理 create, write
				if !w.inEvenOps(event.Op, fs.Write, fs.Create, fs.Rename) {
					continue
				}

				// plg.Infof("filename: %q, op: %s", event.Name, event.Op.String())
				watchFileInfo := w.getWatchFileInfo(event.Name)
				if watchFileInfo == nil {
					continue
				}
				watchFileInfo.IsRename = event.Op == fs.Rename
				watchFileInfo.ChangedFilename = event.Name
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
// 格式:
// ----------------------
// |  WATCH-PATH |  DIR |
// ----------------------
// |  xxxx       |  true |
// -----------------------
func (w *Watch) WatchList() string {
	header := []string{"WATCH-PATH", "DIR"}
	buffer := new(bytes.Buffer)
	buffer.WriteByte('\n')

	table := tw.NewWriter(buffer)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetCenterSeparator("|")
	for path, watchFileInfo := range w.fileMap {
		data := []string{
			path,
			base.ToString(watchFileInfo.IsDir),
		}
		table.Append(data)
	}
	table.Render()
	return buffer.String()
}
