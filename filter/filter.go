package filter

import (
	"github.com/dbcdk/morph/nix"
	"github.com/gobwas/glob"
)

func MatchHosts(allHosts []nix.Host, pattern string) (hosts []nix.Host, err error) {
	g := glob.MustCompile(pattern)

	for _, host := range allHosts {
		if g.Match(host.Name) {
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

	// limit to $limit hosts, making sure not to go out of bounds either
	if limit > 0 && limit < len(hosts) {
		return hosts[:limit]
	} else {
		return hosts
	}
}
