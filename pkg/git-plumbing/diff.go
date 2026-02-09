package git

import (
	"context"
	"strconv"
	"strings"
)

// FileStat represents line-change statistics for a single file.
type FileStat struct {
	Path    string
	Added   int
	Removed int
}

// DiffStat represents aggregate diff statistics.
type DiffStat struct {
	Files []FileStat
	Total FileStat // aggregate totals
}

// DiffCachedStat returns line-change stats for staged (cached) changes.
func (g *Git) DiffCachedStat(ctx context.Context) (*DiffStat, error) {
	lines, err := g.RunLines(ctx, "diff", "--cached", "--numstat")
	if err != nil {
		return nil, err
	}
	return parseNumstat(lines), nil
}

// DiffCachedNames returns file paths with staged changes.
func (g *Git) DiffCachedNames(ctx context.Context) ([]string, error) {
	return g.RunLines(ctx, "diff", "--cached", "--name-only")
}

// parseNumstat parses git diff --numstat output into a DiffStat.
func parseNumstat(lines []string) *DiffStat {
	stat := &DiffStat{}
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		removed, _ := strconv.Atoi(parts[1])
		fs := FileStat{
			Path:    parts[2],
			Added:   added,
			Removed: removed,
		}
		stat.Files = append(stat.Files, fs)
		stat.Total.Added += added
		stat.Total.Removed += removed
	}
	return stat
}
