package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/luskaner/ageServer/common/executor/exec"
)

var waitBeforeRunning time.Duration
var exitBeforeRunning bool
var executableGame string

var execFn = func(options exec.Options) *exec.Result { return options.Exec() }

func rootCmd(args []string) error {
	log.Printf("Arguments: %v", os.Args)
	if exitBeforeRunning {
		log.Printf("Exiting before running the game")
		return nil
	}
	log.Println("Waiting before running the game...")
	time.Sleep(waitBeforeRunning)
	log.Println("Done waiting, launching the game...")
	options := exec.Options{
		File:       executableGame,
		ShowWindow: true,
		Pid:        true,
		Args:       args,
	}
	if result := execFn(options); !result.Success() {
		return fmt.Errorf("failed to start the game: %s", result.Err)
	} else {
		log.Printf("Started the game with PID %d\n", result.Pid)
	}
	return nil
}

func setupFlags() {
	if flag.Lookup("waitBeforeRunning") == nil {
		flag.DurationVar(&waitBeforeRunning, "waitBeforeRunning", 10*time.Second, "Wait time before running the game")
		flag.BoolVar(&exitBeforeRunning, "exitBeforeRunning", false, "Exit without running the game")
	}
}

func Execute() error {
	log.Printf("Arguments: %s", os.Args)
	setupFlags()
	flag.Parse()
	return rootCmd(flag.Args())
}
