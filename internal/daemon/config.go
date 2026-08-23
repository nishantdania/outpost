package daemon

type Config struct {
	ListenAddr     string
	DatabasePath   string
	Token          string
	LauncherSocket string
}
