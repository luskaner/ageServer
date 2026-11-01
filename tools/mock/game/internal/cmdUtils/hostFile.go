package cmdUtils

import (
	"fmt"
	"os"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/hosts"
)

func HandleHostFile(hostFilePath string) (err error) {
	var file *os.File
	if file, err = os.Open(hostFilePath); err != nil {
		return fmt.Errorf("failed to open host file (%s): %w", hostFilePath, err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	var lines []hosts.Line
	if err, _, lines = hosts.GetAllLines(file); err != nil {
		return fmt.Errorf("failed to read host file (%s): %w", hostFilePath, err)
	}
	for _, line := range lines {
		ipStr := line.IP().String()
		for _, host := range line.Hosts() {
			common.CacheMapping(string(host), ipStr)
		}
	}
	return
}
