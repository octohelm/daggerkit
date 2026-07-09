package main

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"dagger/debian/internal/dagger"
)

func New(
	ctx context.Context,
	// +optional
	container *dagger.Container,
	// +optional
	platform dagger.Platform,
	// +optional
	version string,
	// apt source base url
	// +optional
	sourceBaseURL string,
) (*DebianContainer, error) {
	if container == nil {
		ds := &DebianSource{
			Version: namedVersion(cmp.Or(version, "13")),
			BaseURL: cmp.Or(sourceBaseURL, "http://deb.debian.org"),
		}

		container = dag.Container(dagger.ContainerOpts{Platform: platform}).
			From(fmt.Sprintf("debian:%s", ds.Version)).
			WithFile(
				"/etc/apt/sources.list.d/debian.sources",
				dag.File("debian.sources", ds.String()),
			)
	}

	dc := &DebianContainer{
		Container: container,
	}

	return dc, nil
}

type DebianContainer struct {
	*dagger.Container
}

func (t *DebianContainer) WithPackageInstalled(ctx context.Context, packages []string) (*DebianContainer, error) {
	if len(packages) == 0 {
		return t, nil
	}

	ctr, err := t.Container.
		WithExec([]string{"apt-get", "update"}).
		WithExec(slices.Concat(
			[]string{"apt-get", "-y", "--no-install-recommends", "install"},
			packages,
		)).
		WithExec([]string{"rm", "-rf", "/var/lib/apt/lists/*"}).
		Sync(ctx)

	if err != nil {
		return nil, err
	}

	return &DebianContainer{Container: ctr}, nil
}

const (
	MISE_INSTALL_PATH = "/usr/local/bin/mise"
	MISE_DATA_DIR     = "/var/mise"
)

func (t *DebianContainer) WithMise(
	ctx context.Context,
	// mise 版本号，留空则安装最新版
	// +optional
	version string,
	// +optional
	miseGithubToken *dagger.Secret,
	// 不共享
	// +optional
	noShared bool,
) (*DebianContainer, error) {
	dc, err := t.WithPackageInstalled(ctx, []string{
		"git",
		"ca-certificates",
		"build-essential",
	})
	if err != nil {
		return nil, err
	}

	platform, err := dc.Container.Platform(ctx)
	if err != nil {
		return nil, err
	}

	ctr := dc.Container.
		WithEnvVariable("MISE_DATA_DIR", MISE_DATA_DIR).
		WithEnvVariable("MISE_CONFIG_DIR", MISE_DATA_DIR).
		WithEnvVariable("MISE_CACHE_DIR", path.Join(MISE_DATA_DIR, "cache")).
		WithEnvVariable("MISE_INSTALL_PATH", MISE_INSTALL_PATH).
		WithEnvVariable("PATH", path.Join(MISE_DATA_DIR, "shims")+":$PATH", dagger.ContainerWithEnvVariableOpts{Expand: true})

	if !noShared {
		ctr = ctr.WithMountedCache(MISE_DATA_DIR, dag.CacheVolume("mise"))
	}

	if miseGithubToken != nil {
		ctr = ctr.WithSecretVariable("MISE_GITHUB_TOKEN", miseGithubToken)
	}

	miseCtr := dag.Container(dagger.ContainerOpts{Platform: platform}).
		From(fmt.Sprintf("ghcr.io/jdx/mise:%s", cmp.Or(version, "latest")))

	ctr, err = ctr.
		WithFile(
			MISE_INSTALL_PATH,
			miseCtr.File("/usr/local/bin/mise"), // https://github.com/jdx/mise/blob/main/packaging/mise/Dockerfile
		).
		Sync(ctx)
	if err != nil {
		return nil, err
	}

	return &DebianContainer{Container: ctr}, nil
}

func (t *DebianContainer) WithMoutedSource(
	ctx context.Context,
	source *dagger.Directory,
	// +optional
	workingDir string,
) (*DebianContainer, error) {
	if workingDir == "" {
		workingDir = "/src"
	}

	ctr := t.Container.
		WithMountedDirectory(workingDir, source).
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

var debianVersions = map[string]string{
	"12": "bookworm",
	"13": "trixie",
}

func namedVersion(version string) string {
	if v, ok := debianVersions[version]; ok {
		return v
	}
	return version
}

type DebianSource struct {
	BaseURL string
	Version string
}

// https://github.com/debuerreotype/docker-debian-artifacts/blob/3355451ec423321fe5ba232dc55c00f3216f6d87/trixie/rootfs.debian-sources
func (s DebianSource) String() string {
	var b strings.Builder

	b.WriteString("Types: deb\n")
	b.WriteString(fmt.Sprintf("URIs: %s/debian\n", s.BaseURL))
	b.WriteString(fmt.Sprintf("Suites: %s %s-updates\n", s.Version, s.Version))
	b.WriteString("Components: main\n")
	b.WriteString("Signed-By: /usr/share/keyrings/debian-archive-keyring.pgp\n")

	b.WriteString("\n")

	b.WriteString("Types: deb\n")
	b.WriteString(fmt.Sprintf("URIs: %s/debian-security\n", s.BaseURL))
	b.WriteString(fmt.Sprintf("Suites: %s-security\n", s.Version))
	b.WriteString("Components: main\n")
	b.WriteString("Signed-By: /usr/share/keyrings/debian-archive-keyring.pgp\n")

	return b.String()
}
