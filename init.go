package pslog

import (
	"gitee.com/xuesongtao/gotool/base"
	"gitee.com/xuesongtao/gotool/xfile"
)

const (
	defaultHandleChange = 100

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
	filePool = xfile.NewFilePool()                          // 文件池
	NoExpire = base.Datetime2TimeObj("9999-12-31 23:59:59") // 不过期
)
