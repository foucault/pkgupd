// Package log provides convenience tiered logging functions
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

// Logger levels
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

// SetLevel sets the log level for the loggers. A level of 0 means
// that all events will be printed. Otherwise only levels greater
// than this will be displayed. The levels are specified
// as LogDebug, LogInfo, LogWarn, LogErr
func SetLevel(level int) {
	if level > LogErr {
		loglevel = LogErr
	} else {
		loglevel = level
	}
}

// Error prints an error message through the logger.
// Its functionality is equivalent to log.Print
func Error(v ...interface{}) {
	if loglevel <= LogErr {
		err.Print(v...)
	}
}

// Errorf prints an error message through the logger.
// Its functionality is equivalent to log.Printf
func Errorf(format string, v ...interface{}) {
	if loglevel <= LogErr {
		err.Printf(format, v...)
	}
}

// Errorln prints an error message through the logger.
// Its functionality is equivalent to log.Println
func Errorln(v ...interface{}) {
	if loglevel <= LogErr {
		err.Println(v...)
	}
}

// ErrorFatal prints a fatal error message through the logger.
// Its functionality is equivalent to log.Fatal
func ErrorFatal(v ...interface{}) {
	err.Fatal(v...)
}

// ErrorFatalf prints a fatal error message through the logger.
// Its functionality is equivalent to log.Fatalf
func ErrorFatalf(format string, v ...interface{}) {
	err.Fatalf(format, v...)
}

// Info prints an info message through the logger.
// Its functionality is equivalent to log.Print
func Info(v ...interface{}) {
	if loglevel <= LogInfo {
		info.Print(v...)
	}
}

// Infof prints an info message through the logger.
// Its functionality is equivalent to log.Printf
func Infof(format string, v ...interface{}) {
	if loglevel <= LogInfo {
		info.Printf(format, v...)
	}
}

// Infoln prints an info message through the logger.
// Its functionality is equivalent to log.Println
func Infoln(v ...interface{}) {
	if loglevel <= LogInfo {
		info.Println(v...)
	}
}

// Warn prints a warning message through the logger.
// Its functionality is equivalent to log.Print
func Warn(v ...interface{}) {
	if loglevel <= LogWarn {
		warn.Print(v...)
	}
}

// Warnf prints a warning message through the logger.
// Its functionality is equivalent to log.Printf
func Warnf(format string, v ...interface{}) {
	if loglevel <= LogWarn {
		warn.Printf(format, v...)
	}
}

// Warnln prints a warning message through the logger.
// Its functionality is equivalent to log.Println
func Warnln(v ...interface{}) {
	if loglevel <= LogWarn {
		warn.Println(v...)
	}
}

// Debug prints a debug message through the logger.
// Its functionality is equivalent to log.Print
func Debug(v ...interface{}) {
	if loglevel <= LogDebug {
		debug.Print(v...)
	}
}

// Debugf prints a debug message through the logger.
// Its functionality is equivalent to log.Printf
func Debugf(format string, v ...interface{}) {
	if loglevel <= LogDebug {
		debug.Printf(format, v...)
	}
}

// Debugln prints a debug message through the logger.
// Its functionality is equivalent to log.Println
func Debugln(v ...interface{}) {
	if loglevel <= LogDebug {
		debug.Println(v...)
	}
}
