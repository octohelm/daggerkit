package main

import (
	"context"
	"time"

	"dagger/daggerkit/internal/dagger"
)

type Daggerkit struct{}

func (m *Daggerkit) LocalRevInfoVersion(
	ctx context.Context,
	// +defaultPath="/"
	src *dagger.Directory,
) (string, error) {
	return dag.RevInfo(src).Version(ctx)
}

func (m *Daggerkit) RemoteRevInfoVersion(
	ctx context.Context,
) (string, error) {
	src := dag.Git("https://github.com/dagger/otel-go.git").Tag("v1.43.0").Tree()

	return dag.RevInfo(src).Version(ctx)
}

func (m *Daggerkit) DebugMise(
	ctx context.Context,
	// +defaultPath="/"
	source *dagger.Directory,
	// +optional
	debianSourceBaseUrl string,
	// +optional
	miseGithubToken *dagger.Secret,
) (*dagger.Container, error) {
	c := dag.
		DebianContainer(dagger.DebianContainerOpts{
			SourceBaseURL: debianSourceBaseUrl,
		}).
		WithMise(dagger.DebianContainerWithMiseOpts{
			MiseGithubToken: miseGithubToken,
		}).
		WithMoutedSource(source).
		Container()

	return c.
		WithEnvVariable("_RUN_AT", time.Now().String()).
		WithExec([]string{"go", "version"}), nil
}
