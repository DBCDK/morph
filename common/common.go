package common

type MorphOptions struct {
	Version   string
	AssetRoot string

	DryRun          *bool
	JsonOut         *bool
	ConstraintsFlag *[]string
	KeepGCRoot      *bool
	AllowBuildShell *bool

	AsJson              bool
	AskForSudoPasswd    bool
	AttrKey             string
	Deployment          string
	DeploymentsDir      string
	DeployReboot        bool
	DeploySwitchAction  string
	DeployUploadSecrets bool
	ExecuteCommand      []string
	NixBuildTarget      string
	NixBuildTargetFile  string
	OrderingTags        string
	PassCmd             string
	SelectEvery         int
	SelectGlob          string
	SelectLimit         int
	SelectSkip          int
	SelectTags          string
	ShowTrace           bool
	SkipHealthChecks    bool
	SkipPreDeployChecks bool
	Timeout             int
}
