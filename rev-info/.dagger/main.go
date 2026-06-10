// RevInfo —— 从 git 源码目录提取版本信息的 Dagger 模块。
//
// 在容器中执行 git 命令获取提交哈希、时间、tag 和未提交状态，
// 最终生成符合 Go module 规范的 pseudo-version。
package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dagger/rev-info/internal/dagger"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// 使用 alpine/git 作为基础镜像（已包含 git，无需额外安装）。
const baseGitImage = "alpine/git:latest"

// RevInfo 版本信息。
type RevInfo struct {
	// Version 版本号（pseudo-version 或 tag）
	Version string
	// Time 提交时间（RFC3339 格式）
	Time string
	// Short 短提交哈希（12 位）
	Short string
	// Uncommitted 是否有未提交的改动
	Uncommitted bool
}

// String 返回版本字符串。
func (r *RevInfo) String() string {
	return r.Version
}

// New 从源码目录提取版本信息。
func New(
	ctx context.Context,
	// 包含 .git 的源码目录
	source *dagger.Directory,
) (*RevInfo, error) {
	ctr := dag.Container().
		From(baseGitImage).
		WithMountedDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"git", "config", "--global", "--add", "safe.directory", "/src"})

	// 获取完整提交哈希
	rev, err := ctr.WithExec([]string{"git", "log", "-1", "--format=%H"}).Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取提交哈希失败: %w", err)
	}
	rev = strings.TrimSpace(rev)

	// 获取提交时间戳
	tsStr, err := ctr.WithExec([]string{"git", "log", "-1", "--format=%ct"}).Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取提交时间失败: %w", err)
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(tsStr), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("解析提交时间失败: %w", err)
	}
	commitTime := time.Unix(ts, 0).UTC()

	// 获取精确匹配的 tag
	tagOut, _ := ctr.WithExec([]string{"git", "tag", "--points-at", "HEAD"}).Stdout(ctx)
	resolvedTag := pickSemverTag(strings.TrimSpace(tagOut))

	// 如果没有精确匹配，尝试获取最近的祖先 tag
	if resolvedTag == "" {
		descOut, _ := ctr.WithExec([]string{"git", "describe", "--tags", "--abbrev=0"}).Stdout(ctx)
		resolvedTag = pickSemverTag(strings.TrimSpace(descOut))
	}

	// 检查未提交改动
	statusOut, _ := ctr.WithExec([]string{"git", "status", "--porcelain"}).Stdout(ctx)
	uncommitted := len(strings.TrimSpace(statusOut)) > 0

	// 如果没有 tag，尝试从 go.mod 推断主版本号
	if resolvedTag == "" {
		goModOut, _ := ctr.WithExec([]string{"cat", "go.mod"}).Stdout(ctx)
		if goModOut != "" {
			if major := extractPathMajor(strings.TrimSpace(goModOut)); major != "" {
				resolvedTag = major + ".0.0"
			}
		}
	}

	shortRev := rev
	if len(rev) >= 12 {
		shortRev = rev[:12]
	}

	finalVersion := convertVersion(resolvedTag, commitTime, shortRev, uncommitted)

	return &RevInfo{
		Version:     finalVersion,
		Time:        commitTime.Format(time.RFC3339),
		Short:       shortRev,
		Uncommitted: uncommitted,
	}, nil
}

// ParseRevInfo 从版本字符串解析为 RevInfo。
// 支持 pseudo-version（如 "v0.0.0-20231222030512-c093d5e89975"、
// "v0.0.0-dirty.0.20231222022414-5f9d1d44dacc"）以及普通 semver tag。
func ParseRevInfo(version string) (*RevInfo, error) {
	v := strings.TrimSpace(version)
	if v == "" {
		return nil, fmt.Errorf("版本字符串不能为空")
	}

	info := &RevInfo{Version: v}

	if module.IsPseudoVersion(v) {
		info.Uncommitted = strings.Contains(v, "-dirty")

		t, err := module.PseudoVersionTime(v)
		if err != nil {
			return nil, fmt.Errorf("解析 pseudo-version 时间失败: %w", err)
		}
		info.Time = t.Format(time.RFC3339)

		rev, err := module.PseudoVersionRev(v)
		if err != nil {
			return nil, fmt.Errorf("解析 pseudo-version 哈希失败: %w", err)
		}
		info.Short = rev
	}

	return info, nil
}

// pickSemverTag 从 git 命令输出中挑选合法的 semver tag。
func pickSemverTag(out string) string {
	for _, tag := range strings.Split(out, "\n") {
		tag = strings.TrimSpace(tag)
		if semver.Canonical(tag) != "" && !module.IsPseudoVersion(tag) {
			return tag
		}
	}
	return ""
}

// extractPathMajor 从 go.mod 内容中提取主版本号前缀（如 "/v2" → "v2"）。
func extractPathMajor(goModContent string) string {
	for _, line := range strings.Split(goModContent, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		modPath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
		_, pathMajor, ok := module.SplitPathVersion(modPath)
		if ok && pathMajor != "" {
			return module.PathMajorPrefix(pathMajor)
		}
		return ""
	}
	return ""
}

// convertVersion 将给定版本信息转换为 pseudo-version。
func convertVersion(version string, t time.Time, rev string, dirty bool) string {
	exact := true
	base, err := module.PseudoVersionBase(version)
	if err == nil {
		version = base
		exact = false
	}
	if version == "" {
		version = "v0.0.0"
		exact = true
	}
	if dirty {
		version += "-dirty"
		exact = false
	}
	return pseudoVersion(version, t, rev, exact)
}

// pseudoVersion 根据参数生成最终的版本串。
func pseudoVersion(version string, t time.Time, rev string, exact bool) string {
	major := semver.Major(version)
	if major == "" {
		major = "v0"
	}

	if exact {
		build := semver.Build(version)
		segment := fmt.Sprintf("%s-%s", t.UTC().Format(module.PseudoVersionTimestampFormat), rev)
		version = semver.Canonical(version)
		if version == "" {
			version = major + ".0.0"
		}
		return version + "-" + segment + build
	}

	return module.PseudoVersion(major, version, t, rev)
}
