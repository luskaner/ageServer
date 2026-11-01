package server

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/luskaner/ageServer/common/uuid"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/certStore"
)

type AnnounceMessageDataSupportedLatest = common.AnnounceMessageData002

const LatencyMeasurementCount = 3

var (
	tlsDialFn       = tls.Dial
	serverPort      = "443"
	serverPath      = "test"
	newHTTPClientFn = func(serverName string, insecureSkipVerify bool, rootCAs *x509.CertPool) *http.Client {
		tr := &http.Transport{TLSClientConfig: TlsConfig(serverName, insecureSkipVerify, rootCAs)}
		return &http.Client{Transport: tr, Timeout: 1 * time.Second}
	}
	buildURLFn = func(ipAddr net.IP) url.URL {
		return url.URL{Scheme: "https", Host: net.JoinHostPort(ipAddr.String(), serverPort), Path: serverPath}
	}
)

func TlsConfig(serverName string, insecureSkipVerify bool, rootCAs *x509.CertPool) *tls.Config {
	if rootCAs == nil {
		rootCAs = certStore.CertPool()
	}
	return &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
		ServerName:         serverName,
		RootCAs:            rootCAs,
	}
}

func connectToServer(host string, insecureSkipVerify bool, rootCAs *x509.CertPool) (conn *tls.Conn, err error) {
	ips := common.HostOrIpToIps(host)
	var ip string
	if len(ips) == 0 {
		ip = host
	} else {
		ip = ips[0]
	}
	return tlsDialFn("tcp4", net.JoinHostPort(ip, serverPort), TlsConfig(host, insecureSkipVerify, rootCAs))
}

func CheckConnectionFromServer(host string, insecureSkipVerify bool, rootCAs *x509.CertPool) (err error) {
	var conn *tls.Conn
	conn, err = connectToServer(host, insecureSkipVerify, rootCAs)
	if conn != nil {
		defer func() {
			_ = conn.Close()
		}()
	}
	return err
}

func LanServerHost(id uuid.UUID, gameTitle string, host string, insecureSkipVerify bool, rootCAs *x509.CertPool) (ok bool) {
	ipAddrs := common.HostOrIpToIps(host)
	if len(ipAddrs) == 0 {
		return
	}
	for _, ipAddr := range ipAddrs {
		if ok, _, _, _ = LanServerIP(id, gameTitle, net.ParseIP(ipAddr), host, insecureSkipVerify, rootCAs, true); !ok {
			return
		}
	}
	return true
}

func LanServerIP(id uuid.UUID, gameTitle string, ipAddr net.IP, serverName string, insecureSkipVerify bool, rootCAs *x509.CertPool, ignoreLatency bool) (ok bool, serverId uuid.UUID, latency time.Duration, data *AnnounceMessageDataSupportedLatest) {
	client := newHTTPClientFn(serverName, insecureSkipVerify, rootCAs)
	u := buildURLFn(ipAddr)
	if !ignoreLatency {
		var latencyAccumulator time.Duration
		for i := 0; i < LatencyMeasurementCount; i++ {
			start := time.Now()
			req, err := http.NewRequest("HEAD", u.String(), nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", common.UserAgent())
			req.Host = serverName
			if //goland:noinspection ALL
			_, err = client.Do(req); err != nil {
				return
			}
			latencyAccumulator += time.Since(start)
		}
		latency = latencyAccumulator / LatencyMeasurementCount
	}
	req, err := http.NewRequest(
		"GET",
		u.String(),
		nil,
	)
	if err != nil {
		return
	}
	req.Host = serverName
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return
	}
	version := resp.Header.Get(common.VersionHeader)
	serverIdStr := resp.Header.Get(common.IdHeader)
	if version == "" || serverIdStr == "" {
		return
	}
	versionInt, _ := strconv.Atoi(version)
	if versionInt > common.AnnounceVersionLatest {
		return
	}
	var serverIdUuid uuid.UUID
	serverIdUuid, err = uuid.Parse(serverIdStr)
	if err != nil {
		return
	}
	if id != uuid.Nil() && id != serverIdUuid {
		return
	}
	serverId = serverIdUuid
	data = &AnnounceMessageDataSupportedLatest{}
	if err = json.NewDecoder(resp.Body).Decode(data); err != nil {
		return
	}
	if data.GameTitle != gameTitle {
		return
	}
	ok = true
	return
}
