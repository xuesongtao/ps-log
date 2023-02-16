package pslog

import (
	"gitee.com/xuesongtao/gotool/base"
	"gitee.com/xuesongtao/gotool/xfile"
	plg "gitee.com/xuesongtao/ps-log/log"
)

const (
	defaultHandleChange = 100 // 默认记录 offset 变化的次数

	// 控制台 logo
	consoleLogo string = `   
	                
______    ______         |  |    ____     ____  
\____ \  /  ___/  ______ |  |   /  _ \   / ___\ 
|  |_> > \___ \  /_____/ |  |__(  <_> ) / /_/  >
|   __/ /____  >         |____/ \____/  \___  / 
|__|         \/                        /_____/  


`
)

var (
	filePool = xfile.NewFilePool(150)                       // 文件池
	NoExpire = base.Datetime2TimeObj("9999-12-31 23:59:59") // 不过期
)

// SetLogger 设置 logger
func SetLogger(l plg.PsLogger) {
	plg.Plg = l
	xfile.SetLogger(l)
}

// PrintFilePool 是否打印 filePool log
func PrintFilePool(print bool) {
	filePool.Print(print)
}
