package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// CascadeOptions configures the cascade command behavior.
// CascadeOptions controls how the dependency graph is walked and what
// side effects (verify, commit, push) are performed after each pull.
type CascadeOptions struct {
	Root          string // Parent directory containing sibling repos (default: "..")
	DryRun        bool   // Preview the graph walk without executing
	Verify        bool   // Run verify_command after each pull
	Commit        bool   // Auto-commit after each pull
	Push          bool   // Auto-push after each commit
	PR            bool   // Create branch + PR instead of direct commit
	VerifyCommand string // Override verify command (default from vendor.yml or "go build ./...")
}

// CascadeResult summarizes the outcome of a cascade operation.
type CascadeResult struct {
	Order     []string              // Topological order of projects walked
	Updated   []string              // Projects that had files updated by pull
	Current   []string              // Projects that were already current (no changes)
	Failed    []CascadeFailure      // Projects that encountered errors
	Skipped   []string              // Projects skipped (no vendor.yml)
	ProjectResults map[string]*CascadeProjectResult // Per-project details keyed by project name
}

// CascadeProjectResult holds the pull outcome for a single project in the cascade.
type CascadeProjectResult struct {
	Name          string   // Project directory name
	Dir           string   // Absolute path to the project directory
	PullResult    *PullResult // Result from pull operation (nil if skipped/failed)
	VerifyPassed  bool     // Whether verify command succeeded (false if not run)
	VerifyOutput  string   // Stdout/stderr from verify command
	Error         error    // Non-nil if pull or verify failed
}

// CascadeFailure records a project that failed during cascade.
type CascadeFailure struct {
	Project string // Project directory name
	Phase   string // "pull", "verify", "commit", or "push"
	Error   string // Error message
}

// CascadeConfig represents the optional cascade section in vendor.yml.
// CascadeConfig is read from each sibling project's vendor.yml to
// configure cascade behavior for that project.
type CascadeConfig struct {
	Root          string `yaml:"root,omitempty"`
	VerifyCommand string `yaml:"verify_command,omitempty"`
	Commit        bool   `yaml:"commit,omitempty"`
}

// VendorConfigWithCascade extends VendorConfig with optional cascade settings.
// VendorConfigWithCascade is used only for parsing the cascade section from
// vendor.yml files during graph discovery.
type VendorConfigWithCascade struct {
	Vendors []types.VendorSpec `yaml:"vendors"`
	Cascade *CascadeConfig     `yaml:"cascade,omitempty"`
}

// CascadeService implements the cascade command's dependency graph walk.
// CascadeService discovers sibling repos, builds a DAG from vendor relationships,
// topologically sorts, and runs pull in order.
type CascadeService struct {
	rootDir string // The root directory containing sibling repos
}

// NewCascadeService creates a CascadeService rooted at the given directory.
// NewCascadeService expects rootDir to be an absolute path to the parent
// directory containing sibling project directories.
func NewCascadeService(rootDir string) *CascadeService {
	return &CascadeService{rootDir: rootDir}
}

// siblingsWithVendor discovers sibling project directories under rootDir
// that contain a .git-vendor/vendor.yml file. Returns a map of project
// name (directory base name) to absolute directory path.
func (cs *CascadeService) siblingsWithVendor() (map[string]string, error) {
	entries, err := os.ReadDir(cs.rootDir)
	if err != nil {
		return nil, fmt.Errorf("cascade: read root directory %s: %w", cs.rootDir, err)
	}

	siblings := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		dir := filepath.Join(cs.rootDir, name)
		vendorYML := filepath.Join(dir, VendorDir, ConfigFile)
		if _, err := os.Stat(vendorYML); err == nil {
			siblings[name] = dir
		}
	}
	return siblings, nil
}

// loadSiblingConfig loads the VendorConfigWithCascade from a sibling project's vendor.yml.
func loadSiblingConfig(dir string) (*VendorConfigWithCascade, error) {
	store := NewYAMLStore[VendorConfigWithCascade](filepath.Join(dir, VendorDir), ConfigFile, false)
	cfg, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("cascade: load vendor.yml in %s: %w", dir, err)
	}
	return &cfg, nil
}

// BuildDAG constructs a directed acyclic graph from vendor.yml files across
// sibling projects. An edge from project A to project B exists when A vendors
// files from B's URL (matched by checking if any vendor URL contains B's
// directory name or remote URL).
//
// BuildDAG returns:
//   - graph: adjacency list mapping project name to its dependencies
//   - projectDirs: map of project name to absolute directory path
//   - error: if sibling discovery or config loading fails
func (cs *CascadeService) BuildDAG() (graph map[string][]string, projectDirs map[string]string, err error) {
	siblings, err := cs.siblingsWithVendor()
	if err != nil {
		return nil, nil, err
	}

	// Collect all vendor URLs per project and build a URL-to-project lookup.
	// A project is identified by its directory basename.
	type projectInfo struct {
		dir    string
		config *VendorConfigWithCascade
	}

	projects := make(map[string]*projectInfo)
	for name, dir := range siblings {
		cfg, err := loadSiblingConfig(dir)
		if err != nil {
			// Skip projects with unparseable configs rather than aborting cascade
			continue
		}
		projects[name] = &projectInfo{dir: dir, config: cfg}
	}

	// Build reverse map: for each project, figure out which OTHER projects
	// it depends on by examining vendor URLs.
	//
	// Heuristic: if vendor URL ends with "/<sibling-name>" or "/<sibling-name>.git",
	// that vendor is sourcing from that sibling. Also detect local path references
	// like "../<sibling-name>" or "file://../<sibling-name>".
	graph = make(map[string][]string)
	projectDirs = make(map[string]string)

	for name, info := range projects {
		projectDirs[name] = info.dir
		graph[name] = nil // ensure every project appears in graph even with no deps

		for _, vendor := range info.config.Vendors {
			dep := matchSiblingByURL(vendor.URL, projects)
			if dep != "" && dep != name {
				graph[name] = append(graph[name], dep)
			}
		}
	}

	return graph, projectDirs, nil
}

// matchSiblingByURL attempts to identify which sibling project a vendor URL
// references. matchSiblingByURL checks URL path suffixes and local path patterns.
// Returns the project name if matched, empty string otherwise.
func matchSiblingByURL[T any](url string, projects map[string]T) string {
	// Normalize: strip trailing ".git" and trailing slashes
	normalized := strings.TrimSuffix(url, ".git")
	normalized = strings.TrimRight(normalized, "/")

	for name := range projects {
		// Check suffix match: URL ends with /name
		if strings.HasSuffix(normalized, "/"+name) {
			return name
		}

		// Check local path references: ../name, file://../name
		if strings.HasSuffix(normalized, "/../"+name) || normalized == "../"+name || normalized == "./"+name {
			return name
		}

		// Check file:// prefix with relative path
		if strings.HasPrefix(url, "file://") {
			path := strings.TrimPrefix(url, "file://")
			path = strings.TrimRight(path, "/")
			if filepath.Base(path) == name {
				return name
			}
		}
	}
	return ""
}

// TopologicalSort performs a topological sort on the dependency graph using
// Kahn's algorithm. Returns the projects in dependency order (dependencies
// first) or an error if a cycle is detected.
//
// TopologicalSort expects graph to contain all projects as keys, even if
// they have no dependencies (nil/empty slice).
func TopologicalSort(graph map[string][]string) ([]string, error) {
	// Compute in-degree for each node
	inDegree := make(map[string]int)
	for node := range graph {
		if _, ok := inDegree[node]; !ok {
			inDegree[node] = 0
		}
		for _, dep := range graph[node] {
			inDegree[dep] = inDegree[dep] // ensure dep exists
		}
	}

	// Edges go from dependent -> dependency, but for Kahn's we need
	// to process nodes with no incoming edges first. Here "incoming edge"
	// means some other project depends on this one.
	//
	// Rebuild in-degree: for each edge A -> B (A depends on B),
	// A has an incoming edge FROM B in the reversed sense. But for topo sort
	// we want to process B before A, so:
	// - In Kahn's on the original graph (A -> B means "A needs B"),
	//   in-degree counts how many nodes point TO a node.
	//   Edge A -> B means B has in-degree incremented? No.
	//
	// Clarification: graph[A] = [B] means A depends on B.
	// For topological order (B before A), we treat the edge as A -> B
	// where B must come first. In Kahn's, we start with nodes that have
	// no dependencies (nothing they depend on).
	//
	// Actually, we need to reverse: in standard Kahn's with edges pointing
	// from dependency to dependent, start with nodes having in-degree 0.
	// Our graph has edges from dependent to dependency (A depends on B: A -> B).
	// We need B before A, so we reverse edges: B -> A.

	// Recompute: in the reversed graph, in-degree = number of dependencies
	inDegree = make(map[string]int)
	reverseAdj := make(map[string][]string) // dependency -> list of dependents

	for node := range graph {
		inDegree[node] = 0
	}
	// Also ensure all dependency targets are in the graph
	for _, deps := range graph {
		for _, dep := range deps {
			if _, ok := inDegree[dep]; !ok {
				inDegree[dep] = 0
			}
		}
	}

	for node, deps := range graph {
		inDegree[node] += len(deps) // node depends on len(deps) things
		for _, dep := range deps {
			reverseAdj[dep] = append(reverseAdj[dep], node)
		}
	}

	// Start with nodes that have no dependencies (in-degree 0 in our scheme)
	var queue []string
	for node, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, node)
		}
	}

	// Sort queue for deterministic output
	sortStrings(queue)

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		// For each project that depends on this node, decrement in-degree
		dependents := reverseAdj[node]
		sortStrings(dependents)
		for _, dependent := range dependents {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(order) != len(inDegree) {
		// Find nodes involved in cycle for error message
		var cycleNodes []string
		for node, deg := range inDegree {
			if deg > 0 {
				cycleNodes = append(cycleNodes, node)
			}
		}
		sortStrings(cycleNodes)
		return nil, fmt.Errorf("cascade: dependency cycle detected among: %s", strings.Join(cycleNodes, ", "))
	}

	return order, nil
}

// sortStrings sorts a string slice in place for deterministic output.
func sortStrings(s []string) {
	// Simple insertion sort — cascade graph sizes are small (< 20 projects)
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

// Cascade walks the dependency graph in topological order and runs pull
// in each project directory. Cascade respects context cancellation for
// Ctrl+C support.
//
// Flow:
//  1. Build DAG from sibling vendor.yml files
//  2. Topological sort (error on cycles)
//  3. Walk in order: cd into each project and run pull
//  4. Optionally: verify, commit, push after each pull
//  5. Return summary of results
func (cs *CascadeService) Cascade(ctx context.Context, opts CascadeOptions) (*CascadeResult, error) {
	graph, projectDirs, err := cs.BuildDAG()
	if err != nil {
		return nil, err
	}

	if len(graph) == 0 {
		return &CascadeResult{}, nil
	}

	order, err := TopologicalSort(graph)
	if err != nil {
		return nil, err
	}

	result := &CascadeResult{
		Order:          order,
		ProjectResults: make(map[string]*CascadeProjectResult),
	}

	if opts.DryRun {
		// Dry run: populate result with project info without executing
		for _, name := range order {
			dir := projectDirs[name]
			result.ProjectResults[name] = &CascadeProjectResult{
				Name: name,
				Dir:  dir,
			}
		}
		return result, nil
	}

	// Walk in topological order
	for _, name := range order {
		// Check context cancellation before each project
		if err := ctx.Err(); err != nil {
			result.Failed = append(result.Failed, CascadeFailure{
				Project: name,
				Phase:   "pull",
				Error:   "cancelled",
			})
			continue
		}

		dir := projectDirs[name]
		projResult := &CascadeProjectResult{
			Name: name,
			Dir:  dir,
		}
		result.ProjectResults[name] = projResult

		// Run pull in the project directory by invoking git-vendor pull
		pullResult, pullErr := cs.runPullInProject(ctx, dir)
		projResult.PullResult = pullResult

		if pullErr != nil {
			projResult.Error = pullErr
			result.Failed = append(result.Failed, CascadeFailure{
				Project: name,
				Phase:   "pull",
				Error:   pullErr.Error(),
			})
			continue
		}

		// Determine if anything was updated
		if pullResult != nil && pullResult.Updated > 0 {
			result.Updated = append(result.Updated, name)
		} else {
			result.Current = append(result.Current, name)
		}

		// Optional: verify
		if opts.Verify {
			verifyCmd := opts.VerifyCommand
			if verifyCmd == "" {
				// Try to load from project's vendor.yml cascade config
				cfg, cfgErr := loadSiblingConfig(dir)
				if cfgErr == nil && cfg.Cascade != nil && cfg.Cascade.VerifyCommand != "" {
					verifyCmd = cfg.Cascade.VerifyCommand
				} else {
					verifyCmd = "go build ./..."
				}
			}

			output, verifyErr := runCommandInDir(ctx, dir, verifyCmd)
			projResult.VerifyOutput = output
			if verifyErr != nil {
				projResult.Error = verifyErr
				result.Failed = append(result.Failed, CascadeFailure{
					Project: name,
					Phase:   "verify",
					Error:   verifyErr.Error(),
				})
				continue
			}
			projResult.VerifyPassed = true
		}

		// Optional: commit
		if opts.Commit || opts.PR {
			commitErr := runCommandInDirSimple(ctx, dir, "git add -A && git commit -m \"chore(vendor): cascade pull\"")
			if commitErr != nil {
				// Commit may fail if nothing changed — not a hard error
				// Only record as failure if we know files were updated
				if pullResult != nil && pullResult.Updated > 0 {
					result.Failed = append(result.Failed, CascadeFailure{
						Project: name,
						Phase:   "commit",
						Error:   commitErr.Error(),
					})
				}
			} else if opts.Push {
				pushErr := runCommandInDirSimple(ctx, dir, "git push")
				if pushErr != nil {
					result.Failed = append(result.Failed, CascadeFailure{
						Project: name,
						Phase:   "push",
						Error:   pushErr.Error(),
					})
				}
			}
		}
	}

	return result, nil
}

// runPullInProject runs git-vendor pull in the given project directory.
// runPullInProject creates a temporary Manager rooted at the project dir
// and delegates to PullVendors for consistent behavior with the pull command.
func (cs *CascadeService) runPullInProject(ctx context.Context, dir string) (*PullResult, error) {
	vendorDir := filepath.Join(dir, VendorDir)

	configStore := NewFileConfigStore(vendorDir)
	lockStore := NewFileLockStore(vendorDir)
	gitClient := NewSystemGitClient(Verbose)
	fs := NewRootedFileSystem(dir)

	ui := &SilentUICallback{}

	syncer := NewVendorSyncer(configStore, lockStore, gitClient, fs, nil, vendorDir, ui, nil)
	mgr := NewManagerWithSyncer(syncer)

	pullOpts := PullOptions{}
	return mgr.Pull(ctx, pullOpts)
}

// runCommandInDir executes a shell command string in the given directory,
// returning combined stdout+stderr output and any error.
func runCommandInDir(ctx context.Context, dir, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runCommandInDirSimple executes a shell command in the given directory,
// discarding output. Returns error if the command fails.
func runCommandInDirSimple(ctx context.Context, dir, command string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	return cmd.Run()
}
