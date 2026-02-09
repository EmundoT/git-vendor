package git

import (
	"context"
	"strings"
)

// BranchInfo represents information about a git branch.
type BranchInfo struct {
	Name     string
	Hash     string
	Subject  string
	Upstream string
	Current  bool
}

// Branches returns a list of local branches sorted by most recent commit.
func (g *Git) Branches(ctx context.Context) ([]BranchInfo, error) {
	lines, err := g.RunLines(ctx, "branch", "-vv", "--sort=-committerdate")
	if err != nil {
		return nil, err
	}
	var branches []BranchInfo
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		current := line[0] == '*'
		rest := strings.TrimSpace(line[2:])
		parts := strings.Fields(rest)
		if len(parts) < 2 {
			continue
		}
		bi := BranchInfo{
			Name:    parts[0],
			Hash:    parts[1],
			Current: current,
		}
		remaining := strings.Join(parts[2:], " ")
		if strings.HasPrefix(remaining, "[") {
			idx := strings.Index(remaining, "]")
			if idx != -1 {
				bi.Upstream = remaining[1:idx]
				if idx+2 < len(remaining) {
					bi.Subject = strings.TrimSpace(remaining[idx+2:])
				}
			}
		} else {
			bi.Subject = remaining
		}
		branches = append(branches, bi)
	}
	return branches, nil
}

// CreateBranch creates a new branch at the given start point without checking it out.
// If startPoint is empty, CreateBranch creates the branch at HEAD.
func (g *Git) CreateBranch(ctx context.Context, name, startPoint string) error {
	args := []string{"branch", name}
	if startPoint != "" {
		args = append(args, startPoint)
	}
	return g.RunSilent(ctx, args...)
}

// DeleteBranch deletes a local branch.
// If force is true, DeleteBranch uses -D (force delete even if not fully merged).
// If force is false, DeleteBranch uses -d (safe delete, fails if not merged).
func (g *Git) DeleteBranch(ctx context.Context, name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return g.RunSilent(ctx, "branch", flag, name)
}
