package filter

import (
	"github.com/DBCDK/morph/nix"
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

func hasTag(host nix.Host, tag string) bool {
	for _, hostTag := range host.GetTags() {
		if hostTag == tag {
			return true
		}
	}

	return false
}

func FilterHostsTags(allHosts []nix.Host, selectedTags []string) (hosts []nix.Host) {
	if len(selectedTags) == 0 {
		return allHosts
	}

	for _, host := range allHosts {
		include := true
		for _, selectTag := range selectedTags {
			if !hasTag(host, selectTag) {
				include = false;
				break
			}

		}

		if include {
			hosts = append(hosts, host)
		}
	}

	return
}

// Split a list of hosts into two lists based on whether the hosts contain af specific tag.
func splitByTag(hosts []nix.Host, requiredTag string) (hostsWithTag []nix.Host, hostsWithoutTag []nix.Host) {
	for _, host := range hosts {
		hasTag := false
		for _, tag := range host.Tags {
			if tag == requiredTag {
				hasTag = true
				break
			}
		}

		if hasTag {
			hostsWithTag = append(hostsWithTag, host)
		} else {
			hostsWithoutTag = append(hostsWithoutTag, host)
		}
	}

	return
}

// Sort a list of hosts based on their tags and a prioritized list of tags.
func SortHosts(hosts []nix.Host, ordering nix.HostOrdering) (sortedHosts []nix.Host) {
	remainingHosts := hosts
	var withTag []nix.Host

	// For each ordering tag: Split the list of unsorted hosts into a list with and a list without the tag.
	// Add the "with" hosts to the list of sorted hosts, and repeat the loop with the "without" hosts as the remaning
	// hosts.
	// This ensures each host is only added once, and preserving the original ordering in case multiple hosts contain
	// the same tag.

	for _, orderingTag := range ordering.Tags {
		withTag, remainingHosts = splitByTag(remainingHosts, orderingTag)
		sortedHosts = append(sortedHosts, withTag...)
	}

	// `remainingHosts` is now the list of hosts that didn't match any of the ordering tags. Add them last.
	sortedHosts = append(sortedHosts, remainingHosts...)

	return
}
