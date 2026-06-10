package main

import (
	"context"
	"slices"

	"dagger/debian/internal/dagger"
)

func New(
	// +optional
	container *dagger.Container,
	// +optional
	includeMise bool,
	// +optional
	miseVersion string,
) *DebianContainer {
	if container == nil {
		container = dag.Container().From("debian:13")
	}

	dc := &DebianContainer{
		Container: container,
	}

	if includeMise {
		return dc.withMise(miseVersion)
	}

	return dc
}

type DebianContainer struct {
	*dagger.Container
}

func (t *DebianContainer) WithPackageInstalled(packages []string) *DebianContainer {
	if len(packages) == 0 {
		return t
	}

	c := t.Container.
		WithExec([]string{"apt-get", "update"}).
		WithExec(slices.Concat(
			[]string{"apt-get", "-y", "--no-install-recommends", "install"},
			packages,
		)).
		WithExec([]string{"rm", "-rf", "/var/lib/apt/lists/*"})

	return &DebianContainer{Container: c}
}

const (
	MISE_INSTALL_PATH = "/usr/local/bin/mise"
	MISE_DATA_DIR     = "/var/mise"
)

func (t *DebianContainer) withMise(
	// +optional
	// mise 版本号，留空则安装最新版
	version string,
) *DebianContainer {
	c := t.WithPackageInstalled([]string{
		"curl", "git", "ca-certificates", "build-essential",
	})

	ctr := c.Container.
		WithEnvVariable("MISE_DATA_DIR", MISE_DATA_DIR).
		WithEnvVariable("MISE_CONFIG_DIR", MISE_DATA_DIR).
		WithEnvVariable("MISE_CACHE_DIR", MISE_DATA_DIR+"/cache").
		WithEnvVariable("MISE_INSTALL_PATH", MISE_INSTALL_PATH).
		WithEnvVariable("PATH", MISE_DATA_DIR+"/shims:$PATH", dagger.ContainerWithEnvVariableOpts{Expand: true}).
		WithMountedCache(MISE_DATA_DIR, dag.CacheVolume("mise"))

	if version != "" {
		ctr = ctr.WithEnvVariable("MISE_VERSION", version)
	}

	ctr = ctr.WithExec([]string{"sh", "-c", "curl https://mise.run | sh"})

	return &DebianContainer{Container: ctr}
}

func (t *DebianContainer) WithMoutedSource(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	workingDir string,
) (*DebianContainer, error) {
	if workingDir == "" {
		workingDir = "/src"
	}

	ctr := t.Container.WithMountedDirectory(workingDir, src).WithWorkdir(workingDir)

	ok, err := ctr.Exists(ctx, MISE_INSTALL_PATH)
	if err != nil {
		return nil, err
	}

	if ok {
		ctr = ctr.WithExec([]string{"mise", "trust", "-a"}).
			WithExec([]string{"mise", "install"})
	}

	return &DebianContainer{Container: ctr}, nil
}
