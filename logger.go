package pslog

import "gitee.com/xuesongtao/xlog"

type logger interface {
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
}

var (
	plog logger = xlog.DefaultLogger()
)

// SetLogger 设置 logger
func SetLogger(l logger) {
	plog = l
}
