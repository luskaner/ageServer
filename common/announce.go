package common

const AnnouncePort = 31978

// AnnouncePath is the announcement probe the server answers on every host of a
// title, the route the launcher queries to tell whether the host is up. It is
// one of the server's own routes, under /test so the probe and the API
// library's diagnostics share one namespace.
const AnnouncePath = "test/root"

const AnnounceMulticastGroup = "239.31.97.8"
const AnnounceHeader = Name
const IdHeader = "X-Id"
const VersionHeader = "X-Version"

//goland:noinspection GoUnusedConst
const (
	// AnnounceVersion0 : 1.1.X - v1.4.X
	AnnounceVersion0 = iota
	// AnnounceVersion1 : v1.5.X - v1.10.X
	AnnounceVersion1
	// AnnounceVersion2 (v1.11.X and higher) no longer sends any extra data, '/test/root' is supposed to be queried to get it
	AnnounceVersion2
)

var AnnounceVersionLatest = AnnounceVersion2

// Version 1.0.X no additional data was sent

// AnnounceMessageData000 Empty interface equivalent when no struct was used to allow for a future expansion
// 1.1.X - v1.4.X
type AnnounceMessageData000 struct {
}

// AnnounceMessageData001 adds the games supported as not only age2 is supported
// v1.5.X - v1.10.X
type AnnounceMessageData001 struct {
	GameTitles []string
}

// AnnounceMessageData002 sends a single game title (as announcements are separated by game) and version
// v1.11.X and higher
type AnnounceMessageData002 struct {
	GameTitle string
	Version   string
}
