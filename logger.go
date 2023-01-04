package pslog

import "gitee.com/xuesongtao/xlog"

type PsLogger interface {
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
	Warning(v ...interface{})
	Warningf(format string, v ...interface{})
}

var (
	logger PsLogger = xlog.DefaultLogger().Skip(1)
)

// SetLogger 设置 logger
func SetLogger(l PsLogger) {
	logger = l
}
