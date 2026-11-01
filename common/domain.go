package common

import (
	"fmt"

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
const aoe2MacWorldsEdgeDomain = "arthurlive" + apiWorldsEdge
const CdnAgeOfEmpires = CdnAgeOfEmpiresSubdomain + "." + AgeOfEmpires + dotTld
const playFabSuffix = "." + PlayFabDomain + dotTld
const SubDomainAge2Prefix = "pb"
const stdSubDomainReleasePart = "-live-release"
const aoe4SubDomainPrefix = "aoeliverelease"
const aoe4Marker = "dr"

var SelfSignedCertDomains = []string{relicDomain, "*" + worldsEdge + dotTld, "*." + AgeOfEmpires + dotTld}

var generatedDomainsCache = make(map[string][]string)

var directHostToIPFn = DirectHostToIP

var useInternet = true

// SetUseInternet controls whether common may perform outbound internet access.
// Currently it gates the public DNS enumeration of official release domains in
// generateDomains. Disabling it returns only the statically known release
// domains. It invalidates the generated domains cache as it may have been
// populated with the previous setting.
func SetUseInternet(enabled bool) {
	useInternet = enabled
	generatedDomainsCache = make(map[string][]string)
}

func CertDomains() []string {
	domains := []string{"*" + playFabSuffix}
	domains = append(domains, SelfSignedCertDomains...)
	return domains
}

func SelfSignedCertGame(game string) bool {
	return game != commonGame.AoE4 && game != commonGame.AoM
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
	}
	domains = append(domains, generateDomains(gameId)...)
	return domains
}

func AllHosts(gameId string, withMacOsExclusive bool) (domains []string) {
	domains = GameHosts(gameId, withMacOsExclusive)
	switch gameId {
	case commonGame.AoM:
		domains = append(domains, "c15f9"+playFabSuffix)
	case commonGame.AoE4:
		domains = append(domains, "ed603"+playFabSuffix)
	}
	domains = append(domains, CdnAgeOfEmpires)
	if gameId == commonGame.AoE4 {
		domains = append(domains, Aoe4ApiAgeOfEmpires)
	} else {
		domains = append(domains, ApiAgeOfEmpires)
	}
	return
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
	if useInternet {
		for release := releaseMin + 1; ; release++ {
			if _, err := directHostToIPFn(generateDomainName(release)); err == nil {
				domains = append(domains, generateDomainName(release))
			} else {
				break
			}
		}
	}
	generatedDomainsCache[gameId] = domains
	return
}
