package main

import (
	"game/internal/cmd"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

func main() {
	dir := filepath.Join("logs", "game")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	file, err := os.OpenFile(
		filepath.Join(dir, time.Now().Format("2006-01-02_15-04-05"+".log")),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0666,
	)
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	multiWriter := io.MultiWriter(os.Stdout, file)
	log.SetOutput(multiWriter)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic: %v\nStack Trace:\n%s", r, debug.Stack())
			panic(r)
		}
	}()
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
