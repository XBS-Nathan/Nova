package caddy

// Service wraps the caddy package functions for the lifecycle interface.
type Service struct{}

func (Service) Start() error { return nil }
func (Service) Stop() error  { return nil }
func (Service) Link(siteName, docroot string, upstream Upstream, portProxies []PortProxy) error {
	return Link(siteName, docroot, upstream, portProxies)
}
func (Service) Unlink(siteName string) error { return Unlink(siteName) }
func (Service) Reload() error                { return Reload() }
