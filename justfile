[group('meta')]
default:
    @just --list

develop:
    dagger develop -r

local-rev-info:
    dagger call local-rev-info-version

remote-rev-info:
    dagger call remote-rev-info-version

debug-mise:
    dagger --progress=plain call debug-mise \
        --mise-github-token=env://MISE_GITHUB_TOKEN \
        --debian-source-base-url={{ env("DEBIAN_SOURCE_BASE_URL", "") }} \
        stdout

publish:
    dagger --progress=plain call \
        container \
            --debian-source-base-url={{ env("DEBIAN_SOURCE_BASE_URL", "") }} \
                push \
                    --registry="ghcr.io" \
                    --username=${GH_USERNAME} \
                    --password=env://GH_PASSWORD

clean:
    dagger core engine local-cache prune

dep:
    find . -name "go.mod" -execdir go mod tidy \;

fmt:
    find . -name "go.mod" -execdir go fmt ./... \;
