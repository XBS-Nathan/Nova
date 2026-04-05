package docker

type Service struct {
	ProjectsDir string
}

func (s Service) Up(phpVersions []string) error {
	return Up(s.ProjectsDir, phpVersions)
}
func (s Service) Down() error { return Down() }
func (s Service) Exec(service, workdir string, args ...string) error {
	return Exec(service, workdir, args...)
}
