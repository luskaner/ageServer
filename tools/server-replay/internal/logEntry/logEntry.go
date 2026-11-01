package logEntry

import (
	"crypto/sha512"
	"fmt"
	"log"
	"net"
	"slices"
	"time"

	"github.com/r3labs/diff/v3"
)

var entries []Replays

type Times interface {
	Uptime() time.Duration
}

type Replays interface {
	Times
	fmt.Stringer
	Replay(serverIP net.IP)
	CheckResponse()
}

func Sort() {
	slices.SortStableFunc(entries, func(a, b Replays) int {
		return int(a.Uptime() - b.Uptime())
	})
}

func Add(entry Replays) {
	entries = append(entries, entry)
}

func Replay(serverIP net.IP, ignoreDelays bool) {
	if len(entries) == 0 {
		log.Println("No entries to replay.")
		return
	}
	previousUptime := entries[0].Uptime()
	sleep := func(entry Replays) {}
	if !ignoreDelays {
		sleep = func(entry Replays) {
			sleepTime := entry.Uptime() - previousUptime
			log.Println("Waiting for", sleepTime)
			time.Sleep(sleepTime)
			previousUptime = entry.Uptime()
		}
	}
	log.Println("REPLAYING...")
	for _, entry := range entries {
		sleep(entry)
		log.Println(entry)
		entry.Replay(serverIP)
	}
	log.Println("CHECKING RESPONSES...")
	for _, entry := range entries {
		log.Println(entry)
		entry.CheckResponse()
	}
}

func SameBody(fromHash [64]byte, toBody []byte) bool {
	var hash [64]byte
	if len(toBody) > 0 {
		hash = sha512.Sum512(toBody)
	}
	return hash == fromHash
}

func matchAll[T any](fn func(T) bool, a ...any) bool {
	for _, e := range a {
		eConverted, ok := e.(T)
		if !ok || !fn(eConverted) {
			return false
		}
	}
	return true
}

func ip(val string) bool {
	return net.ParseIP(val) != nil
}

func epoch(epoch float64) bool {
	epochDate := time.Unix(int64(epoch), 0)
	now := time.Now()
	if ceil := now.Add(24 * time.Hour); epochDate.After(ceil) {
		return false
	}
	if bottom := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC); epochDate.Before(bottom) {
		return false
	}
	return true
}

const Iso8601Layout = "2006-01-02T15:04:05.000Z"

func dateIso8601(val string) bool {
	if _, err := time.Parse(Iso8601Layout, val); err != nil {
		return false
	}
	return true
}

func dateRFC3339(val string) bool {
	if _, err := time.Parse(time.RFC3339, val); err != nil {
		return false
	}
	return true
}

func dateRFC1123(val string) bool {
	if _, err := time.Parse(time.RFC1123, val); err != nil {
		return false
	}
	return true
}

const playfabDateFormat = "2006-01-02T15:04:05.000Z"

func datePlayfab(val string) bool {
	if _, err := time.Parse(playfabDateFormat, val); err != nil {
		return false
	}
	return true
}

func CompareJSON(from any, to any) bool {
	changelog, err := diff.Diff(from, to)
	if err != nil {
		log.Println("Could not diff body as JSON")
		return false
	} else if len(changelog) > 0 {
		var finalChangeLog diff.Changelog
		for _, change := range changelog {
			if change.Type != "update" ||
				(!matchAll[float64](epoch, change.From, change.To) &&
					!matchAll(ip, change.From, change.To) &&
					!matchAll(dateIso8601, change.From, change.To) &&
					!matchAll(dateRFC3339, change.From, change.To) &&
					!matchAll(dateRFC1123, change.From, change.To) &&
					!matchAll(datePlayfab, change.From, change.To)) {
				finalChangeLog = append(finalChangeLog, change)
			}
		}
		if len(finalChangeLog) > 0 {
			log.Printf("%#v", finalChangeLog)
			return false
		}
	}
	return true
}
