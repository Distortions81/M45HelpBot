package cwlog

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentLoggingAfterFileFailure(t *testing.T) {
	CloseCWLog()
	t.Cleanup(CloseCWLog)
	file, err := os.Create(filepath.Join(t.TempDir(), "closed.log"))
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	logMu.Lock()
	LogDesc = file
	logMu.Unlock()
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Go(func() { DoLog("file failure falls back to stdout") })
	}
	workers.Wait()
	logMu.Lock()
	defer logMu.Unlock()
	if LogDesc != nil {
		t.Fatal("failed log descriptor was not released")
	}
}

func TestLoggingDuringClose(t *testing.T) {
	t.Chdir(t.TempDir())
	StartCWLog()
	t.Cleanup(CloseCWLog)
	var workers sync.WaitGroup
	workers.Go(func() { DoLog("before or during shutdown") })
	workers.Go(CloseCWLog)
	workers.Wait()
	DoLog("after shutdown")
}
