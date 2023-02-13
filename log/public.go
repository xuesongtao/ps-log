package log

func Info(v ...interface{}) {
	Plg.Info(v...)
}

func Infof(format string, v ...interface{}) {
	Plg.Infof(format, v...)
}

func Error(v ...interface{}) {
	Plg.Error(v...)
}

func Errorf(format string, v ...interface{}) {
	Plg.Errorf(format, v...)
}

func Warning(v ...interface{}) {
	Plg.Warning(v...)
}

func Warningf(format string, v ...interface{}) {
	Plg.Warningf(format, v...)
}
