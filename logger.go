package pslog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
)

type PsLogger interface {
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
	Warning(v ...interface{})
	Warningf(format string, v ...interface{})
}

var (
	plg   PsLogger = newDefaultLogger()
	once sync.Once
)

// SetLogger 设置 pl
func SetLogger(l PsLogger) {
	once.Do(func() {
		plg = l
	})
}

type defaultLogger struct {
	log *log.Logger
}

func newDefaultLogger() *defaultLogger {
	return &defaultLogger{
		log: log.New(os.Stderr, "", log.LstdFlags),
	}
}

func (d *defaultLogger) Info(v ...interface{}) {
	d.log.Println(append([]interface{}{"[INFO] " + d.getPrefix(3)}, v...)...)
}

func (d *defaultLogger) Infof(format string, v ...interface{}) {
	d.log.Printf("[INFO] "+d.getPrefix(3)+" "+format, v...)
}

func (d *defaultLogger) Error(v ...interface{}) {
	d.log.Println(append([]interface{}{"[ERRO] " + d.getPrefix(3)}, v...)...)
}

func (d *defaultLogger) Errorf(format string, v ...interface{}) {
	d.log.Printf("[ERRO] "+d.getPrefix(3)+" "+format, v...)
}

func (d *defaultLogger) Warning(v ...interface{}) {
	d.log.Println(append([]interface{}{"[WARN] " + d.getPrefix(3)}, v...)...)
}

func (d *defaultLogger) Warningf(format string, v ...interface{}) {
	d.log.Printf("[WARN] "+d.getPrefix(3)+" "+format, v...)
}

func (d *defaultLogger) Fatal(v ...interface{}) {
	d.log.Println(append([]interface{}{"[ERRO] " + d.getPrefix(3)}, v...)...)
	os.Exit(1)
}

func (d *defaultLogger) Fatalf(format string, v ...interface{}) {
	d.log.Printf("[ERRO] "+d.getPrefix(3)+" "+format, v...)
	os.Exit(1)
}

func (d *defaultLogger) Panic(v ...interface{}) {
	d.log.Println(append([]interface{}{"[ERRO] " + d.getPrefix(3)}, v...)...)
	panic(fmt.Sprint(v...))
}

func (d *defaultLogger) Panicf(format string, v ...interface{}) {
	d.log.Printf("[ERRO] "+d.getPrefix(3)+" "+format, v...)
	panic(fmt.Sprintf(format, v...))
}

func (d *defaultLogger) getPrefix(skip int) string {
	file, line := d.callInfo(skip)
	return file + ":" + strconv.Itoa(line)
}

func (d *defaultLogger) callInfo(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "", 0
	}
	file = filepath.Base(file)
	return file, line
}
