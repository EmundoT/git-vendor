package git

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ListTree lists files and directories at a given ref and path.
// Returns entries with "/" suffix for directories.
func (g *Git) ListTree(ctx context.Context, ref, subdir string) ([]string, error) {
	target := ref
	if target == "" {
		target = "HEAD"
	}
	args := []string{"ls-tree", target}
	if subdir != "" && subdir != "." {
		args = append(args, strings.TrimSuffix(subdir, "/")+"/")
	}
	out, err := g.Run(ctx, args...)
	if err != nil && subdir != "" {
		args = []string{"ls-tree", target, strings.TrimSuffix(subdir, "/")}
		out, err = g.Run(ctx, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("git ls-tree failed: %w", err)
	}
	return parseTreeOutput(out, subdir), nil
}

// parseTreeOutput parses git ls-tree output into a sorted list of entries.
func parseTreeOutput(output, subdir string) []string {
	lines := strings.Split(output, "\n")
	var items []string
	for _, l := range lines {
		parts := strings.Fields(l)
		if len(parts) < 4 {
			continue
		}
		objType := parts[1]
		fullPath := strings.Join(parts[3:], " ")

		relName := fullPath
		if subdir != "" && subdir != "." {
			cleanSub := strings.TrimSuffix(subdir, "/") + "/"
			if !strings.HasPrefix(fullPath, cleanSub) {
				continue
			}
			relName = strings.TrimPrefix(fullPath, cleanSub)
		}
		if relName == "" {
			continue
		}
		if objType == "tree" {
			items = append(items, relName+"/")
		} else {
			items = append(items, relName)
		}
	}
	sort.Strings(items)
	return items
}
