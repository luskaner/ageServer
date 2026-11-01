package router

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/luskaner/ageServer/common"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/routes/game/login"
)

const authCacheValidity = 30 * 24 * time.Hour

func AuthMiddlewareOffline(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value("user").(models.User)
		now := r.Context().Value("time").(time.Time)
		var valid bool
		_ = u.GetAuth().WithReadOnly(func(deadline *time.Time) error {
			valid = !(*deadline).IsZero() && (*deadline).After(now)
			return nil
		})
		if valid {
			next.ServeHTTP(w, r)
		} else {
			login.PlatformLoginError(now, w)
		}
	})
}

func AuthMiddleware(next http.HandlerFunc, gameId string, cached bool) http.Handler {
	hosts := common.GameHosts(gameId, true)
	hostToIp := make(map[string]string)
	for _, host := range hosts {
		if ip, err := common.DirectHostToIP(host); err == nil {
			hostToIp[host] = ip
		}
	}
	caCert, err := os.ReadFile(filepath.Join(models.EtcFolder, "cacert.pem"))
	if err != nil {
		panic(err)
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		panic("Cannot parse internal CA store")
	}
	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}
	dialer := &net.Dialer{
		Timeout: 1 * time.Second,
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, dialErr := net.SplitHostPort(addr)
			if dialErr != nil {
				return dialer.DialContext(ctx, network, addr)
			}
			if ip, ok := hostToIp[host]; ok {
				addr = net.JoinHostPort(ip, port)
			} else {
				return nil, fmt.Errorf("host %s not resolved", host)
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := r.Context().Value("time").(time.Time)
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			login.PlatformLoginError(t, w)
			return
		}
		if len(bodyBytes) == 0 && r.PostForm != nil {
			bodyBytes = []byte(r.PostForm.Encode())
		}
		errorHandler := func() {
			if cached {
				AuthMiddlewareOffline(next).ServeHTTP(w, r)
			} else {
				login.PlatformLoginError(t, w)
			}
		}
		doRequest := func(c *http.Client, r *http.Request) (respAny i.A, ok bool) {
			resp, localErr := c.Do(r)
			if localErr != nil {
				errorHandler()
				return
			}
			defer func(Body io.ReadCloser) {
				_ = Body.Close()
			}(resp.Body)
			if resp.StatusCode != http.StatusOK {
				errorHandler()
				return
			}
			respBody, localErr := io.ReadAll(resp.Body)
			if localErr != nil {
				errorHandler()
				return
			}
			localErr = json.Unmarshal(respBody, &respAny)
			if localErr != nil || len(respAny) == 0 {
				errorHandler()
				return
			}

			errorCode, ok := respAny[0].(float64)
			if !ok {
				errorHandler()
				return
			}

			if int(errorCode) != common.ErrSuccess {
				// Re-encode the upstream error payload instead of copying the
				// raw response body into ours: the bytes come from a remote
				// service and must not be reflected verbatim.
				i.JSON(&w, respAny)
				return
			}

			ok = true
			return
		}
		authReq := r.Clone(context.Background())
		authReq.RequestURI = ""
		authReq.RemoteAddr = ""
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		authReq.Header.Set("User-Agent", common.UserAgent())
		authReq.URL.Scheme = "https"
		authReq.URL.Host = r.Host
		authReq.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		authReq.ContentLength = int64(len(bodyBytes))
		authReq.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewBuffer(bodyBytes)), nil
		}
		authReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		authReq.Form = nil
		jar, err := cookiejar.New(nil)
		if err != nil {
			panic(err)
		}
		client := &http.Client{
			Timeout:   1 * time.Second,
			Jar:       jar,
			Transport: transport,
		}
		authResp, ok := doRequest(client, authReq)
		callNum := 2
		if !ok {
			return
		}

		upstreamSessionId, extractOk := safeString(authResp, 1)
		if !extractOk {
			errorHandler()
			return
		}
		defer func() {
			data := url.Values{}
			data.Add("callNum", strconv.Itoa(callNum))
			data.Add("connect_id", upstreamSessionId)
			data.Add("sessionID", upstreamSessionId)
			encodedData := data.Encode()
			dataReader := strings.NewReader(encodedData)
			logoutReq := r.Clone(context.Background())
			logoutReq.RequestURI = ""
			logoutReq.RemoteAddr = ""
			logoutReq.Header.Set("User-Agent", common.UserAgent())
			logoutReq.URL.Scheme = "https"
			logoutReq.URL.Host = r.Host
			logoutReq.URL.RawQuery = ""
			logoutReq.URL.Path = "/game/login/logout"
			logoutReq.Body = io.NopCloser(dataReader)
			logoutReq.ContentLength = int64(dataReader.Len())
			logoutReq.GetBody = func() (io.ReadCloser, error) {
				reader := strings.NewReader(encodedData)
				return io.NopCloser(reader), nil
			}
			logoutReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			logoutReq.Form = nil
			_, _ = doRequest(client, logoutReq)
		}()
		u := r.Context().Value("user").(models.User)
		if cached {
			_ = u.GetAuth().WithReadWrite(func(deadline *time.Time) error {
				*deadline = t.Add(authCacheValidity)
				return nil
			})
		}
		req, err := http.NewRequest(
			"GET",
			fmt.Sprintf(
				"https://%s/%s",
				r.Host,
				"game/item/getInventoryByProfileIDs",
			),
			nil,
		)
		if err != nil {
			errorHandler()
			return
		}
		req.Header.Set("User-Agent", common.UserAgent())
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Pragma", "no-cache")
		q := req.URL.Query()
		q.Add("callNum", strconv.Itoa(callNum))
		q.Add("connect_id", upstreamSessionId)
		q.Add("sessionID", upstreamSessionId)
		q.Add("profileIDs", fmt.Sprintf("[%d]", int32(safeNestedFloat(authResp, 5))))
		req.URL.RawQuery = q.Encode()

		resp, ok := doRequest(client, req)
		if !ok {
			return
		}
		callNum++
		itemsRaw, itemsOk := safeNestedArray(resp, 1)
		if !itemsOk {
			errorHandler()
			return
		}
		_ = u.GetItems().WithReadWrite(func(items *map[int32]*models.MainItem) error {
			clear(*items)
			for _, itemRaw := range itemsRaw {
				itemData, itemOk := itemRaw.(i.A)
				if !itemOk {
					continue
				}
				item := models.NewMainItemFromRaw(itemData)
				(*items)[item.GetId()] = item
			}
			return nil
		})
		next.ServeHTTP(w, r)
	})
}

// safeString extracts a string from an i.A at the given index.
func safeString(data i.A, index int) (string, bool) {
	if index < 0 || index >= len(data) {
		return "", false
	}
	s, ok := data[index].(string)
	return s, ok
}

// safeNestedFloat extracts a float64 from the nested structure, that is
// data[index], then its "A" member, then [0], then its "A" member, then [1].
func safeNestedFloat(data i.A, index int) float64 {
	if index < 0 || index >= len(data) {
		return 0
	}
	arr1, ok := data[index].(i.A)
	if !ok || len(arr1) < 1 {
		return 0
	}
	arr2, ok := arr1[0].(i.A)
	if !ok || len(arr2) < 2 {
		return 0
	}
	val, ok := arr2[1].(float64)
	if !ok {
		return 0
	}
	return val
}

// safeNestedArray extracts an i.A from data at the given index.
func safeNestedArray(data i.A, index int) (i.A, bool) {
	if index < 0 || index >= len(data) {
		return nil, false
	}
	arr, ok := data[index].(i.A)
	return arr, ok
}
