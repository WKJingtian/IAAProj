package applog

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
)

var (
	mu      sync.Mutex
	name    = "app"
	logPath string
	logFile *os.File
	logger  = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
)

type syncFileWriter struct {
	file *os.File
}

func (w *syncFileWriter) Write(p []byte) (int, error) {
	n, err := w.file.Write(p)
	if err != nil {
		return n, err
	}
	if err := w.file.Sync(); err != nil {
		return n, err
	}
	return n, nil
}

func Init(serviceName string) error {
	return InitWithPath(serviceName, "log.txt")
}

func InitWithPath(serviceName string, fileName string) error {
	trimmedName := strings.TrimSpace(serviceName)
	if trimmedName == "" {
		trimmedName = "app"
	}

	trimmedFile := strings.TrimSpace(fileName)
	if trimmedFile == "" {
		trimmedFile = "log.txt"
	}

	baseDir, err := executableDir()
	if err != nil {
		baseDir = "."
	}
	fullPath := filepath.Join(baseDir, trimmedFile)

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file failed: %w", err)
	}

	output := io.MultiWriter(os.Stdout, &syncFileWriter{file: f})

	mu.Lock()
	oldFile := logFile
	name = trimmedName
	logPath = fullPath
	logFile = f
	logger.SetOutput(output)
	logger.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(output)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	mu.Unlock()

	if oldFile != nil {
		_ = oldFile.Close()
	}

	Infof("logger initialized: %s", fullPath)
	return nil
}

func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return logPath
}

func Close() error {
	mu.Lock()
	f := logFile
	logFile = nil
	logPath = ""
	logger.SetOutput(os.Stdout)
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	mu.Unlock()

	if f == nil {
		return nil
	}

	var closeErr error
	if err := f.Sync(); err != nil {
		closeErr = errors.Join(closeErr, err)
	}
	if err := f.Close(); err != nil {
		closeErr = errors.Join(closeErr, err)
	}
	return closeErr
}

func CatchPanic() {
	if rec := recover(); rec != nil {
		Errorf("panic: %v\n%s", rec, string(debug.Stack()))
		_ = Close()
		panic(rec)
	}
}

func Infof(format string, args ...any) {
	logf("INFO", format, args...)
}

func Errorf(format string, args ...any) {
	logf("ERROR", format, args...)
}

func logf(level string, format string, args ...any) {
	mu.Lock()
	currentName := name
	mu.Unlock()

	logger.Printf("[%s][%s] %s", currentName, level, fmt.Sprintf(format, args...))
}

func executableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	if dir == "" {
		return "", errors.New("empty executable dir")
	}
	return dir, nil
}
