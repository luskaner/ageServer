package common

import (
	"fmt"
	"net"
	"slices"
	"strings"

	commonGame "github.com/luskaner/ageServer/common/game"
)

const Tld = "com"
const dotTld = "." + Tld
const linkMainDomainSuffix = "link"
const ApiSubdomainSuffix = "-api"
const SubDomain = "aoe" + ApiSubdomainSuffix
const RelicMainDomain = "relic" + linkMainDomainSuffix
const relicDomain = SubDomain + "." + RelicMainDomain + dotTld
const WorldsEdgeMainDomain = "worldsedge" + linkMainDomainSuffix
const worldsEdge = "." + WorldsEdgeMainDomain
const apiWorldsEdge = ApiSubdomainSuffix + worldsEdge + dotTld
const PlayFabDomain = "playfabapi"
const AgeOfEmpires = "ageofempires"
const ApiAgeOfEmpiresSubdomain = "api"
const CdnAgeOfEmpiresSubdomain = "cdn"
const apiAgeOfEmpiresSuffix = "." + AgeOfEmpires + dotTld
const ApiAgeOfEmpires = ApiAgeOfEmpiresSubdomain + apiAgeOfEmpiresSuffix
const Aoe4ApiAgeOfEmpires = ApiAgeOfEmpiresSubdomain + "-" + aoe4Marker + apiAgeOfEmpiresSuffix
const aoe2MacSubDomainPrefix = "arthurlive"
const aoe2MacReleasePart = "-release"
const aoe2MacReleaseMin = 2
const aoe2MacWorldsEdgeDomain = aoe2MacSubDomainPrefix + apiWorldsEdge
const CdnAgeOfEmpires = CdnAgeOfEmpiresSubdomain + "." + AgeOfEmpires + dotTld
const playFabSuffix = "." + PlayFabDomain + dotTld
const SubDomainAge2Prefix = "pb"
const stdSubDomainReleasePart = "-live-release"
const aoe4SubDomainPrefix = "aoeliverelease"
const aoe4Marker = "dr"

var SelfSignedCertDomains = []string{relicDomain, "*" + worldsEdge + dotTld, "*." + AgeOfEmpires + dotTld}

var generatedDomainsCache = make(map[string][]string)

var generatedMacOsDomainsCache []string

var directHostToIPFn = DirectHostToIP

var useInternet = true

// SetUseInternet controls whether common may perform outbound internet access.
// Currently it gates the public DNS enumeration of official release domains in
// generateDomains and generateMacOsDomains. Disabling it returns only the
// statically known release domains. It invalidates the generated domains caches
// as they may have been populated with the previous setting.
func SetUseInternet(enabled bool) {
	useInternet = enabled
	generatedDomainsCache = make(map[string][]string)
	generatedMacOsDomainsCache = nil
}

func CertDomains() []string {
	domains := []string{"*" + playFabSuffix}
	domains = append(domains, SelfSignedCertDomains...)
	return domains
}

// CertMode names one of the two certificate pairs the server can listen with.
// A listener serves every enabled game; the mode only selects which pair it
// presents, because a title of the other class may reject the handshake.
type CertMode string

const (
	// CertModeModern is the CA-signed leaf (cert.pem/key.pem, with cacert.pem as
	// the CA). AoE IV and AoM require it because they reject a self-signed leaf.
	CertModeModern CertMode = "modern"
	// CertModeLegacy is the self-signed leaf (selfsigned_cert.pem and
	// selfsigned_key.pem), the class AoE I-III: DE have always accepted.
	CertModeLegacy CertMode = "legacy"
)

// GameCertMode is the certificate class a title needs. The other class may be
// accepted once the compatibility spike (docs/multi-game-per-ip.md, M0) proves
// it, which is why PreferredModes exists instead of a bare boolean.
func GameCertMode(game string) CertMode {
	if game == commonGame.AoE4 || game == commonGame.AoM {
		return CertModeModern
	}
	return CertModeLegacy
}

// PreferredModes returns the certificate classes a title prefers, its own class
// first and the other one after it.
func PreferredModes(game string) []CertMode {
	if GameCertMode(game) == CertModeModern {
		return []CertMode{CertModeModern, CertModeLegacy}
	}
	return []CertMode{CertModeLegacy, CertModeModern}
}

func SelfSignedCertGame(game string) bool {
	return GameCertMode(game) == CertModeLegacy
}

// NormalizeHost is the form every host comparison uses: lower case, no port and
// no trailing dot.
func NormalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimSuffix(host, ".")
}

// BuildDomainToGameIDs maps every host a title talks to the titles that answer
// on it. Hosts of games whose API tree is not served by this process go to
// unserved instead of to the map: a disabled API host must answer 404 rather
// than fall back to the single enabled game.
//
// It must be built after SetUseInternet, because the generated release domains
// depend on it.
func BuildDomainToGameIDs(apiEnabled bool) (served map[string][]string, unserved map[string]bool) {
	served = map[string][]string{}
	unserved = map[string]bool{}
	for _, g := range commonGame.SupportedGames.ToSlice() {
		var hosts []string
		if apiEnabled {
			hosts = AllHosts(g, true)
		} else {
			hosts = RootHosts(g, true)
			for _, h := range ApiHosts(g, true) {
				unserved[NormalizeHost(h)] = true
			}
		}
		for _, h := range hosts {
			key := NormalizeHost(h)
			if key == "" {
				continue
			}
			if !slices.Contains(served[key], g) {
				served[key] = append(served[key], g)
			}
		}
	}
	for key := range served {
		slices.Sort(served[key])
	}
	return served, unserved
}

func GameHosts(gameId string, withMacOsExclusive bool) (domains []string) {
	switch gameId {
	case commonGame.AoE4:
		for i := 1; i <= 2; i++ {
			domains = append(domains, fmt.Sprintf("%s%d%s", aoe4SubDomainPrefix, i, apiWorldsEdge))
		}
		fallthrough
	case commonGame.AoE1, commonGame.AoE2, commonGame.AoE3:
		domains = append(domains, relicDomain, SubDomain+worldsEdge+dotTld)
	case commonGame.AoM:
		domains = []string{"athens-live" + apiWorldsEdge}
	}
	if gameId == commonGame.AoE2 && withMacOsExclusive {
		domains = append(domains, aoe2MacWorldsEdgeDomain)
		domains = append(domains, generateMacOsDomains()...)
	}
	domains = append(domains, generateDomains(gameId)...)
	return domains
}

// AllHosts is every domain the game talks to: the ones the root server answers
// and the ones the API answers. It is composed from the two halves so that the
// halves and the whole cannot diverge.
func AllHosts(gameId string, withMacOsExclusive bool) (domains []string) {
	domains = RootHosts(gameId, withMacOsExclusive)
	domains = append(domains, ApiHosts(gameId, withMacOsExclusive)...)
	return
}

// RootHosts are the domains the root server answers itself: the game's own
// hosts, its PlayFab API and the CDN.
func RootHosts(gameId string, withMacOsExclusive bool) (domains []string) {
	domains = GameHosts(gameId, withMacOsExclusive)
	switch gameId {
	case commonGame.AoM:
		domains = append(domains, "c15f9"+playFabSuffix)
	case commonGame.AoE4:
		domains = append(domains, "ed603"+playFabSuffix)
	}
	domains = append(domains, CdnAgeOfEmpires)
	return
}

// ApiHosts are the domains of the Age of Empires API: the mods and match
// content tree. They can be answered by this server or by a standalone API
// server, which is why they are separate from the rest.
func ApiHosts(gameId string, _ bool) (domains []string) {
	if gameId == commonGame.AoE4 {
		return []string{Aoe4ApiAgeOfEmpires}
	}
	return []string{ApiAgeOfEmpires}
}

func generateDomains(gameId string) (domains []string) {
	if cache, ok := generatedDomainsCache[gameId]; ok {
		return cache
	}
	var prefix string
	var releaseMin int
	var subDomainReleasePart string
	switch gameId {
	case commonGame.AoE2:
		prefix = SubDomainAge2Prefix
		releaseMin = 7
		subDomainReleasePart = stdSubDomainReleasePart
	case commonGame.AoE4:
		prefix = aoe4Marker
		releaseMin = 10
		subDomainReleasePart = "-activerelease"
	case commonGame.AoM:
		prefix = "andromeda"
		releaseMin = 22
		subDomainReleasePart = stdSubDomainReleasePart
	default:
		return
	}
	generateDomainName := func(release int) string {
		return fmt.Sprintf("%s%s%d%s", prefix, subDomainReleasePart, release, apiWorldsEdge)
	}
	for release := 1; release <= releaseMin; release++ {
		domains = append(domains, generateDomainName(release))
	}
	domains = appendProbedReleases(domains, releaseMin, generateDomainName)
	generatedDomainsCache[gameId] = domains
	return
}

// appendProbedReleases appends the release domains that exist beyond the
// statically known ones. It stops at the first release that does not resolve,
// so it requires outbound DNS access.
func appendProbedReleases(domains []string, releaseMin int, generateDomainName func(int) string) []string {
	if !useInternet {
		return domains
	}
	for release := releaseMin + 1; ; release++ {
		if _, err := directHostToIPFn(generateDomainName(release)); err == nil {
			domains = append(domains, generateDomainName(release))
		} else {
			break
		}
	}
	return domains
}

func macOsDomainName(release int) string {
	return fmt.Sprintf("%s%s%d%s", aoe2MacSubDomainPrefix, aoe2MacReleasePart, release, apiWorldsEdge)
}

func staticMacOsDomains() (domains []string) {
	for release := 1; release <= aoe2MacReleaseMin; release++ {
		domains = append(domains, macOsDomainName(release))
	}
	return
}

func generateMacOsDomains() (domains []string) {
	if generatedMacOsDomainsCache != nil {
		return generatedMacOsDomainsCache
	}
	domains = appendProbedReleases(staticMacOsDomains(), aoe2MacReleaseMin, macOsDomainName)
	generatedMacOsDomainsCache = domains
	return
}
