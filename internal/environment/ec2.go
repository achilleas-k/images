package environment

type EC2 struct {
	BaseEnvironment
}

func (p *EC2) GetPackages() []string {
	return []string{"cloud-init"}
}

func (p *EC2) GetServices() []string {
	return []string{
		"cloud-init.service",
		"cloud-config.service",
		"cloud-final.service",
		"cloud-init-local.service",
	}
}

func (p *EC2) GetKernelArgs() []string {
	return []string{"ro", "no_timer_check", "console=ttyS0,115200n8", "net.ifnames=0"}
}
