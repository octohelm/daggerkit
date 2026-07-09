package main

import (
	"context"
	"fmt"

	"dagger/daggerkit/internal/dagger"
)

func (*Daggerkit) Container(
	ctx context.Context,
	// +defaultPath="/"
	// +ignore=["target"]
	source *dagger.Directory,
	// +optional
	debianSourceBaseURL string,
) *DaggerkitContainer {
	return &DaggerkitContainer{
		Source:              source,
		DebianSourceBaseURL: debianSourceBaseURL,
	}
}

type DaggerkitContainer struct {
	Source              *dagger.Directory
	DebianSourceBaseURL string
}

func (c *DaggerkitContainer) Build(
	ctx context.Context,
	// +optional
	version string,
	// +optional
	platform dagger.Platform,
) (*dagger.Container, error) {
	if version == "" {
		ver, err := dag.RevInfo(c.Source).Version(ctx)
		if err != nil {
			return nil, err
		}
		version = ver
	}

	ctr := dag.DebianContainer(
		dagger.DebianContainerOpts{
			Platform:      platform,
			SourceBaseURL: c.DebianSourceBaseURL,
		}).
		WithMise(dagger.DebianContainerWithMiseOpts{
			NoShared: true,
		}).
		Container()

	return ctr.
		WithExec([]string{
			"mise", "use", "--global", "dagger", "just",
		}).
		Sync(ctx)
}

func (c *DaggerkitContainer) Push(
	ctx context.Context,
	// +default="ghcr.io"
	registry string,
	// +default="octohelm/daggerkit"
	name string,
	// +optional
	username string,
	// +optional
	password *dagger.Secret,
) (string, error) {
	version, err := dag.RevInfo(c.Source).Version(ctx)
	if err != nil {
		return "", err
	}

	imageName := fmt.Sprintf("%s/%s:%s", registry, name, version)

	amd64ctr, err := c.Build(ctx, version, "linux/amd64")
	if err != nil {
		return "", err
	}

	arm64ctr, err := c.Build(ctx, version, "linux/arm64")
	if err != nil {
		return "", err
	}

	if registry != "" && password != nil && username != "" {
		amd64ctr = amd64ctr.WithRegistryAuth(registry, username, password)
	}

	return amd64ctr.
		With(labeledImageSource).
		Publish(ctx, imageName, dagger.ContainerPublishOpts{
			PlatformVariants: []*dagger.Container{
				arm64ctr.With(labeledImageSource),
			},
		})
}

func labeledImageSource(ctr *dagger.Container) *dagger.Container {
	return ctr.WithLabel("org.opencontainers.image.source", "https://github.com/octohelm/daggerkit")
}
