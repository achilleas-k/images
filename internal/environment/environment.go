package environment

import "github.com/osbuild/images/pkg/rpmmd"

type Environment interface {
	GetPackages() []string
	GetRepos() []rpmmd.RepoConfig
	GetServices() []string
	GetKernelArgs() []string
}

type BaseEnvironment struct {
	Repos []rpmmd.RepoConfig
}

func (p BaseEnvironment) GetPackages() []string {
	return []string{}
}

func (p BaseEnvironment) GetRepos() []rpmmd.RepoConfig {
	return p.Repos
}

func (p BaseEnvironment) GetServices() []string {
	return []string{}
}

func (p BaseEnvironment) GetKernelArgs() []string {
	return []string{}
}
