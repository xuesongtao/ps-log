package pslog

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"strings"

	xf "gitee.com/xuesongtao/gotool/xfile"
	plg "gitee.com/xuesongtao/ps-log/log"
)

const (
	defaultLogDir = "log" // 项目中默认的 log 目录
)

type LogDir struct {
	Namespace   string   // log 的命名空间
	Name        string   // 项目中 log 的目录名, 为空时, 默认 defaultLogDir
	TargetNames []string // 目标文件名
}

func (l *LogDir) init() {
	if l.Name == "" {
		l.Name = defaultLogDir
	}
}

func (l *LogDir) valid() error {
	if l.Namespace == "" {
		return errors.New("namespace is null")
	}

	if l.Name == "" {
		return errors.New("name is null")
	}

	if len(l.TargetNames) == 0 {
		return errors.New("targetNames is null")
	}
	return nil
}

// ProjectSrc 项目根目录
type ProjectSrc struct {
	LogDir  *LogDir
	SrcPath string // 根目录 path, 必须为绝对路径, 如: /root/demo/src
}

// Project 项目根目录
type Project struct {
	LogDir      *LogDir
	ProjectPath string // 项目 path, 必须为绝对路径, 如: /root/demo/src/demo
}

type ProjectLog struct {
	ProjectName string
	Namespace   string // log 的命名空间
	LogPath     string // 项目的 log path
}

type LogPath struct {
	excludeProjectDir map[string]bool // 排除的项目路径
}

func NewLogPath() *LogPath {
	return &LogPath{
		excludeProjectDir: make(map[string]bool),
	}
}

// ParseSrc 收集 src 下的 go 项目, 注意: src.SrcPath 必须为绝对路径
// src 目录即为项目根目录, 目录结构如下:
//
// src
// ├── demo1
// │   ├── app
// │   ├── boot
// │   ├── config
// │   ├── log
// │   ├── main.go
// └── demo2
// │   ├── app
// │   ├── boot
// │   ├── config
// │   ├── log
// │   ├── main.go
//
func (l *LogPath) ParseSrc(src *ProjectSrc) []*ProjectLog {
	if src == nil {
		return nil
	}

	projectLogPaths := make([]*ProjectLog, 0) // 解析所有目标文件
	// 解析 src 下的项目
	projectDirs, err := ioutil.ReadDir(src.SrcPath)
	if err != nil {
		plg.Error("os.ReadDir is failed, err: ", err)
		return nil
	}

	// 遍历项目
	for _, projectDir := range projectDirs {
		appName := projectDir.Name() // 项目名
		logPaths := l.ParseLogPath(&Project{
			LogDir: &LogDir{
				Namespace:   src.LogDir.Namespace,
				TargetNames: src.LogDir.TargetNames,
			},
			ProjectPath: filepath.Join(src.SrcPath, appName),
		})
		projectLogPaths = append(projectLogPaths, logPaths...)
	}
	return projectLogPaths
}

// ParseLogPaths 解析项目 log path
func (l *LogPath) ParseLogPath(project *Project) []*ProjectLog {
	if l.excludePath(project.ProjectPath) {
		return nil
	}
	if project.LogDir == nil {
		plg.Error("logDir is nil")
		return nil
	}
	project.LogDir.init()
	if err := project.LogDir.valid(); err != nil {
		plg.Error("project.LogDir.valid is failed, err:", err)
		return nil
	}

	projectLogPaths := make([]*ProjectLog, 0, len(project.LogDir.TargetNames)) // 解析所有目标文件
	// 解析目标文件
	for _, targetName := range project.LogDir.TargetNames {
		logPath := filepath.Join(project.ProjectPath, project.LogDir.Name, targetName)
		if !xf.Exists(logPath) {
			continue
		}
		projectLogPaths = append(projectLogPaths, &ProjectLog{
			ProjectName: filepath.Base(project.ProjectPath),
			Namespace:   project.LogDir.Namespace,
			LogPath:     logPath,
		})
	}
	return projectLogPaths
}

// SetExcludeProjectDir 设置排除的项目目录
func (l *LogPath) SetExcludeProjectDir(paths ...string) {
	if l.excludeProjectDir == nil {
		l.excludeProjectDir = make(map[string]bool, len(paths))
	}
	for _, path := range paths {
		path = strings.TrimRight(path, "/")
		l.excludeProjectDir[path] = true
	}
}

// excludePath 判断是否被排除
func (l *LogPath) excludePath(projectDir string) bool {
	if len(l.excludeProjectDir) == 0 {
		return false
	}
	projectDir = strings.TrimRight(projectDir, "/")
	_, ok := l.excludeProjectDir[projectDir]
	return ok
}
