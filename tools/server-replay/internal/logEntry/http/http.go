package http

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/logger/serverCommunication/request"
	clientHttp "github.com/luskaner/ageServer/server-replay/internal/client/http"
	"github.com/luskaner/ageServer/server-replay/internal/logEntry"
	"github.com/r3labs/diff/v3"
)

type response struct {
	statusCode int
	header     http.Header
	body       []byte
	latency    time.Duration
}

type Request struct {
	response
	ignored bool
	data    request.Read
}

func cookies(header http.Header) []*http.Cookie {
	var c []*http.Cookie
	if cookie, err := http.ParseCookie(header.Get("Set-Cookie")); err == nil {
		c = cookie
	}
	for _, cookie := range c {
		cookie.MaxAge = 0
	}
	return c
}

func delHeader(header1 *http.Header, header2 *http.Header, values ...string) {
	for _, v := range values {
		header1.Del(v)
		header2.Del(v)
	}
}

func (r *Request) CheckResponse() {
	if r.ignored {
		log.Println("SKIP")
		return
	}
	if r.response.statusCode != r.data.StatusCode {
		log.Println("Expected status code", r.data.StatusCode, "got", r.response.statusCode)
		return
	}
	outHeaders := r.data.Out.Headers.Clone()
	responseHeaders := r.response.header.Clone()
	delHeader(&outHeaders, &responseHeaders, "Set-Cookie", "Date", "Content-Length", "Last-Modified")
	changelog, err := diff.Diff(outHeaders, responseHeaders)
	if err != nil {
		log.Println("Could not compare headers")
		return
	} else if len(changelog) > 0 {
		log.Printf("%#v", changelog)
		return
	}
	outCookies := cookies(outHeaders)
	responseCookies := cookies(responseHeaders)
	changelog, err = diff.Diff(outCookies, responseCookies)
	if err != nil {
		log.Println("Could not compare cookies")
		return
	} else if len(changelog) > 0 {
		log.Printf("%#v", changelog)
		return
	}
	if len(r.data.Out.Body.Body) > 0 {
		var actualData any
		if err = json.Unmarshal(r.response.body, &actualData); err == nil {
			var expectedData any
			if err = json.Unmarshal(r.data.Out.Body.Body, &expectedData); err == nil {
				if logEntry.CompareJSON(actualData, expectedData) {
					log.Println("OK")
				}
				return
			}
		}
	}
	if !logEntry.SameBody(r.data.Out.BodyHash.BodyHash, r.response.body) {
		log.Println("Body hash does not match")
		return
	}
	log.Println("OK")
}

func (r *Request) String() string {
	return r.data.In.Method + " " + r.data.In.Url.String()
}

func (r *Request) Replay(serverIP net.IP) {
	// wss.Connection will handle the /wss/ instead
	if r.data.In.Url.Path == "/wss/" ||
		r.data.In.Url.Path == "/cacert.pem" ||
		((r.data.Url.Host == common.ApiAgeOfEmpires || r.data.Url.Host == common.Aoe4ApiAgeOfEmpires) && r.data.In.Url.Path != "/test" && r.data.In.Url.Path != "/textmoderation") ||
		(r.data.Url.Host == common.CdnAgeOfEmpires && r.data.In.Url.Path != "/test" && r.data.In.Url.Path != "/aoe/athens-server-status.json") {
		log.Println("SKIP")
		r.ignored = true
		return
	}
	originalHost := r.data.In.Url.Host
	modifiedUrl := *r.data.In.Url
	modifiedUrl.Host = serverIP.String()
	var body io.Reader
	if len(r.data.In.Body.Body) > 0 {
		body = bytes.NewReader(r.data.In.Body.Body)
	}
	req, err := http.NewRequest(r.data.Method, modifiedUrl.String(), body)
	if err != nil {
		log.Println("Could not build request:", err)
		r.ignored = true
		return
	}
	req.Header = r.data.In.Headers
	req.Host = originalHost
	client := clientHttp.GetOrNew(r.data.Sender.Sender)
	now := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		// A transport failure must not kill the whole replay run; the entry
		// will simply be reported as a mismatch in CheckResponse.
		log.Println("Request failed:", err)
		r.response.statusCode = 0
		r.response.header = http.Header{}
		r.response.body = nil
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	data, err := io.ReadAll(resp.Body)
	r.latency = time.Since(now)
	if err != nil {
		log.Println(err)
	} else {
		r.response.body = data
	}
	r.response.statusCode = resp.StatusCode
	r.response.header = resp.Header
}

func (r *Request) Uptime() time.Duration {
	return r.data.Uptime.Uptime
}

func NewRequest(data request.Read) *Request {
	return &Request{data: data}
}
