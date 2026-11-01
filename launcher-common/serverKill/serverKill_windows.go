package serverKill

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/process"
)

func Do(path string) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Second,
	}
	ips := common.ResolveUnspecifiedIps()
	if len(ips) == 0 {
		return fmt.Errorf("could not resolve local IPs")
	}
	for _, ip := range ips {
		req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/shutdown", ip), nil)
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "")
		req.Header.Set("User-Agent", common.UserAgent())
		//goland:noinspection ALL
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
	}
	pid, proc, err := process.Process(path)
	if err != nil {
		return err
	} else if proc == nil {
		return nil
	}
	if process.WaitForProcess(proc, new(2*time.Second)) {
		return nil
	}
	if err = process.KillPidProc(pid, proc); err != nil {
		return err
	}
	return nil
}
