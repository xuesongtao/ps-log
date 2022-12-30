package pslog

// FileInfo
type FileInfo struct {
	IsDir  bool   // 是否为目录
	Path   string // 文件路径
	Offset int64  // 记录文件偏移量
}
