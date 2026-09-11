package cwlog

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var LogDesc *os.File
var logMu sync.Mutex

/* Normal CW log */
func DoLog(text string) {
	ctime := time.Now()
	_, filename, line, _ := runtime.Caller(1)

	date := fmt.Sprintf("%2v:%2v.%2v", ctime.Hour(), ctime.Minute(), ctime.Second())
	buf := fmt.Sprintf("%v: %15v:%5v: %v\n", date, filepath.Base(filename), line, text)
	fmt.Print(buf)
	logMu.Lock()
	defer logMu.Unlock()
	if LogDesc == nil {
		return
	}
	if _, err := LogDesc.WriteString(buf); err != nil {
		fmt.Println("DoLog: WriteString failure")
		LogDesc.Close()
		LogDesc = nil
		return
	}
}

/* Prep everything for the cw log */
func StartCWLog() {
	t := time.Now()

	/* Create our log file names */
	logName := fmt.Sprintf("log/%v-%v-%v.log", t.Day(), t.Month(), t.Year())

	/* Make log directory */
	errr := os.MkdirAll("log", os.ModePerm)
	if errr != nil {
		fmt.Print(errr.Error())
		return
	}

	/* Open log files */
	bdesc, errb := os.OpenFile(logName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	/* Handle file errors */
	if errb != nil {
		fmt.Printf("An error occurred when attempting to create cw log. Details: %s", errb)
		return
	}

	/* Save descriptors, open/closed elsewhere */
	logMu.Lock()
	defer logMu.Unlock()
	if LogDesc != nil {
		LogDesc.Close()
	}
	LogDesc = bdesc
}

func CloseCWLog() {
	logMu.Lock()
	defer logMu.Unlock()
	if LogDesc != nil {
		LogDesc.Close()
		LogDesc = nil
	}
}
