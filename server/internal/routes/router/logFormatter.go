package router

import (
	"fmt"
	"html"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/luskaner/ageServer/server/internal/logger"
)

func formatDurationWithDays(d time.Duration) string {
	d = d.Truncate(time.Second)
	day := 24 * time.Hour
	days := d / day
	d -= days * day
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second
	result := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	if days > 0 {
		result = fmt.Sprintf("%dd %s", days, result)
	}
	return result
}

func logFormatter(writer io.Writer, params handlers.LogFormatterParams) {
	host, _, err := net.SplitHostPort(params.Request.RemoteAddr)
	if err != nil {
		host = params.Request.RemoteAddr
	}
	uri := params.Request.RequestURI
	if params.Request.ProtoMajor == 2 && params.Request.Method == "CONNECT" {
		uri = params.Request.Host
	}
	if uri == "" {
		uri = params.Request.URL.RequestURI()
	}
	t := params.TimeStamp.UTC().Sub(logger.StartTime)
	var buf strings.Builder
	buf.WriteString(host)
	buf.WriteString(" - ")
	buf.WriteString(formatDurationWithDays(t))
	buf.WriteString(" - ")
	buf.WriteString(params.Request.Method)
	buf.WriteString(" ")
	buf.WriteString(html.EscapeString(uri))
	buf.WriteString(" - ")
	buf.WriteString(strconv.Itoa(params.StatusCode))
	buf.WriteString(" ")
	buf.WriteString(strconv.Itoa(params.Size))
	buf.WriteString("\n")
	_, _ = writer.Write([]byte(buf.String()))
}
