# 列出所有仓库入口
[group('meta')]
default:
    @just --list --list-submodules

develop:
    dagger develop -r

local-rev-info:
    dagger call local-rev-info-version

remote-rev-info:
    dagger call remote-rev-info-version

debug-mise:
    dagger call debug-mise stdout
