package main

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"slices"

	"dagger/debian/internal/dagger"
)

func New(
	ctx context.Context,
	// +optional
	container *dagger.Container,
	// +optional
	includeMise bool,
	// +optional
	miseVersion string,
	// +optional
	miseGithubToken *dagger.Secret,
) (*DebianContainer, error) {
	if container == nil {
		container = dag.Container().From("debian:13")
	}

	dc := &DebianContainer{
		Container: container,
	}

	if includeMise {
		return dc.withMise(ctx, miseVersion, miseGithubToken)
	}

	return dc, nil
}

type DebianContainer struct {
	*dagger.Container
}

func (t *DebianContainer) WithPackageInstalled(packages []string) *DebianContainer {
	if len(packages) == 0 {
		return t
	}

	ctr := t.Container.
		WithExec([]string{"apt-get", "update"}).
		WithExec(slices.Concat(
			[]string{"apt-get", "-y", "--no-install-recommends", "install"},
			packages,
		)).
		WithExec([]string{"rm", "-rf", "/var/lib/apt/lists/*"})

	return &DebianContainer{Container: ctr}
}

const (
	MISE_INSTALL_PATH = "/usr/local/bin/mise"
	MISE_DATA_DIR     = "/var/mise"
)

func (t *DebianContainer) withMise(
	ctx context.Context,
	// +optional
	// mise 版本号，留空则安装最新版
	version string,
	// +optional
	miseGithubToken *dagger.Secret,
) (*DebianContainer, error) {
	c := t.WithPackageInstalled([]string{
		"curl",
		"git",
		"ca-certificates",
		"build-essential",
	})

	ctr := c.Container.
		WithEnvVariable("MISE_DATA_DIR", MISE_DATA_DIR).
		WithEnvVariable("MISE_CONFIG_DIR", MISE_DATA_DIR).
		WithEnvVariable("MISE_CACHE_DIR", path.Join(MISE_DATA_DIR, "cache")).
		WithEnvVariable("MISE_INSTALL_PATH", MISE_INSTALL_PATH).
		WithEnvVariable("PATH", path.Join(MISE_DATA_DIR, "shims")+":$PATH", dagger.ContainerWithEnvVariableOpts{Expand: true}).
		WithMountedCache(MISE_DATA_DIR, dag.CacheVolume("mise"))

	if miseGithubToken != nil {
		ctr = ctr.WithSecretVariable("MISE_GITHUB_TOKEN", miseGithubToken)
	}

	mise := dag.Container().From(fmt.Sprintf("ghcr.io/jdx/mise:%s", cmp.Or(version, "latest")))

	ctr = ctr.WithFile(
		MISE_INSTALL_PATH,
		// https://github.com/jdx/mise/blob/main/packaging/mise/Dockerfile
		mise.File("/usr/local/bin/mise"),
	)

	return &DebianContainer{Container: ctr}, nil
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

	ctr := t.Container.
		WithMountedDirectory(workingDir, src).
		WithWorkdir(workingDir)

	ok, err := ctr.Exists(ctx, MISE_INSTALL_PATH)
	if err != nil {
		return nil, err
	}

	if ok {
		ctr = ctr.
			WithExec([]string{"mise", "trust", "-a"}).
			WithExec([]string{"mise", "install"})
	}

	installed, err := ctr.Sync(ctx)
	if err != nil {
		return nil, err
	}

	return &DebianContainer{Container: installed}, nil
}
