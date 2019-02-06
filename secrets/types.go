package secrets

import "fmt"
import "strings"

type Secret struct {
	Source      string
	Destination string
	Owner       Owner
	Permissions string
	Action      []string
	MkDirs      bool
}

type Owner struct {
	Group string
	User  string
}

func (s *Secret) String() string {
	var string_repr strings.Builder

	fmt.Fprintf(&string_repr, "`%s` -> `%s`, with:\n\tPermissions: %s:%s, %s\n\tCreate remote directories: %t",
		s.Source, s.Destination, s.Owner.User, s.Owner.Group, s.Permissions, s.MkDirs)

	if len(s.Action) > 0 {
		fmt.Fprintf(&string_repr, "\n\tAction: `%s`", strings.Join(s.Action, " "))
	}

	return string_repr.String()
}
