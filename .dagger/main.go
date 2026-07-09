package main

import (
	"context"
	"dagger/daggerkit/internal/dagger"
	"os"
	"time"
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
	src *dagger.Directory,
) (*dagger.Container, error) {
	c := dag.
		DebianContainer(dagger.DebianContainerOpts{
			IncludeMise:     true,
			MiseGithubToken: dag.SetSecret("MISE_GITHUB_TOKEN", os.Getenv("MISE_GITHUB_TOKEN")),
		}).
		WithMoutedSource(src).
		Container()

	return c.
		WithEnvVariable("_RUN_AT", time.Now().String()).
		WithExec([]string{"go", "version"}), nil
}
