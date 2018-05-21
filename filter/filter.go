package filter

import (
	"git-platform.dbc.dk/platform/morph/nix"
	"github.com/gobwas/glob"
)

func MatchHosts(allHosts []nix.Host, pattern string) (hosts []nix.Host, err error) {
	g := glob.MustCompile(pattern)

	for _, host := range allHosts {
		if g.Match(host.TargetHost) {
			hosts = append(hosts, host)
		}
	}

	return
}

func FilterHosts(allHosts []nix.Host, skip int, every int, limit int) (hosts []nix.Host) {
	// skip first $skip hosts
	if skip >= len(allHosts) {
		return hosts
	}
	for index, host := range allHosts[skip:] {
		// select every $every hosts
		if index%every == 0 {
			hosts = append(hosts, host)
		}
	}

	// limit to $limit hosts
	if limit > 0 {
		return hosts[:limit]
	} else {
		return hosts
	}
}
