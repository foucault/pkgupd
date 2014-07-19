package log

import glog "log"
import "os"
import "fmt"
import "syscall"
import "unsafe"

const ioctlReadTermios = 0x5401
const ioctlWriteTermios = 0x5402
const ansiBold = 1
const (
	ansiBlack = iota
	ansiRed
	ansiGreen
	ansiYellow
	ansiBlue
	ansiMagenta
	ansiCyan
	ansiWhite
)
const (
	_ = iota
	LogDebug
	LogInfo
	LogWarn
	LogErr
)

func isTerminal(fd int) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		ioctlReadTermios, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return err == 0
}

func genericNew(prefix string, color int) *glog.Logger {
	hlt := ""
	rst := ""
	if isTerminal(int((os.Stderr).Fd())) {
		hlt = fmt.Sprintf("\033[3%dm\033[%dm", color, ansiBold)
		rst = "\033[0m"
	}
	realPrefix := fmt.Sprintf("[%s%s%s] ", hlt, prefix, rst)
	return glog.New(os.Stderr, realPrefix, glog.LstdFlags)
}

func newError() *glog.Logger {
	return genericNew("ERR", ansiRed)
}

func newInfo() *glog.Logger {
	return genericNew("INF", ansiBlue)
}

func newDebug() *glog.Logger {
	return genericNew("DBG", ansiMagenta)
}

func newWarning() *glog.Logger {
	return genericNew("WRN", ansiYellow)
}

var err = newError()
var info = newInfo()
var debug = newDebug()
var warn = newWarning()
var loglevel = LogWarn

func SetLevel(level int) {
	if level > LogErr {
		loglevel = LogErr
	} else {
		loglevel = level
	}
}

func Error(v ...interface{}) {
	if loglevel <= LogErr {
		err.Print(v...)
	}
}

func Errorf(format string, v ...interface{}) {
	if loglevel <= LogErr {
		err.Printf(format, v...)
	}
}

func Errorln(v ...interface{}) {
	if loglevel <= LogErr {
		err.Println(v...)
	}
}

func ErrorFatal(v ...interface{}) {
	err.Fatal(v...)
}

func ErrorFatalf(format string, v ...interface{}) {
	err.Fatalf(format, v...)
}

func Info(v ...interface{}) {
	if loglevel <= LogInfo {
		info.Print(v...)
	}
}

func Infof(format string, v ...interface{}) {
	if loglevel <= LogInfo {
		info.Printf(format, v...)
	}
}

func Infoln(v ...interface{}) {
	if loglevel <= LogInfo {
		info.Println(v...)
	}
}

func Warn(v ...interface{}) {
	if loglevel <= LogWarn {
		warn.Print(v...)
	}
}

func Warnf(format string, v ...interface{}) {
	if loglevel <= LogWarn {
		warn.Printf(format, v...)
	}
}

func Warnln(v ...interface{}) {
	if loglevel <= LogWarn {
		warn.Println(v...)
	}
}

func Debug(v ...interface{}) {
	if loglevel <= LogDebug {
		debug.Print(v...)
	}
}

func Debugf(format string, v ...interface{}) {
	if loglevel <= LogDebug {
		debug.Printf(format, v...)
	}
}

func Debugln(v ...interface{}) {
	if loglevel <= LogDebug {
		debug.Println(v...)
	}
}
