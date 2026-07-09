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
    dagger --progress=plain -vv call debug-mise stdout

clean:
    dagger core engine local-cache prune

dep:
    find . -name "go.mod" -execdir go mod tidy \;

fmt:
    find . -name "go.mod" -execdir go fmt ./... \;
