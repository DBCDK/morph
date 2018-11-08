package secrets

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
