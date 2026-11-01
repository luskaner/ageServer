package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/server-replay/internal/decoder"
	"github.com/luskaner/ageServer/server-replay/internal/logEntry"
	"github.com/spf13/cobra"
)

var serverLogPath string
var ignoreDelays bool
var serverIP net.IP
var Version string

var (
	rootCmd = &cobra.Command{
		Use:   filepath.Base(os.Args[0]),
		Short: "serverReplay replays the server's communication log",
		Run: func(_ *cobra.Command, _ []string) {
			if f, err := os.OpenFile(serverLogPath, os.O_RDONLY, 0666); err != nil {
				panic(err)
			} else {
				defer func(f *os.File) {
					_ = f.Close()
				}(f)
				if err = decoder.Decode(f); err != nil {
					panic(err)
				}
				fmt.Println("Start the server, wait for initialization and press ENTER to continue...")
				var temp string
				_, _ = fmt.Scanln(&temp)
				logEntry.Replay(serverIP, ignoreDelays)
			}
		},
	}
)

func Execute() error {
	rootCmd.Version = Version
	rootCmd.Flags().BoolVarP(&ignoreDelays, "ignoreDelays", "d", false, "Ignore delays between requests. Could run much faster but may not be adequate in some situations.")
	rootCmd.Flags().StringVarP(&serverLogPath, "serverLogPath", "l", "", "Path to server communication log.")
	rootCmd.Flags().IPVarP(&serverIP, "serverIP", "i", net.ParseIP("127.0.0.1"), "IP of the server to use.")
	if err := rootCmd.MarkFlagRequired("serverLogPath"); err != nil {
		panic(err)
	}
	rootCmd.Version = Version
	return rootCmd.Execute()
}
