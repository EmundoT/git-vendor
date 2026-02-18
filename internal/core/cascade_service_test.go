package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestTopologicalSort_LinearChain verifies TopologicalSort produces the correct
// order for a simple linear dependency chain: C depends on B depends on A.
func TestTopologicalSort_LinearChain(t *testing.T) {
	graph := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"B"},
	}

	order, err := TopologicalSort(graph)
	if err != nil {
		t.Fatalf("TopologicalSort returned error for linear chain: %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("TopologicalSort returned %d items, want 3", len(order))
	}

	// A must come before B, B must come before C
	indexOf := make(map[string]int)
	for i, name := range order {
		indexOf[name] = i
	}

	if indexOf["A"] >= indexOf["B"] {
		t.Errorf("A (index %d) must come before B (index %d)", indexOf["A"], indexOf["B"])
	}
	if indexOf["B"] >= indexOf["C"] {
		t.Errorf("B (index %d) must come before C (index %d)", indexOf["B"], indexOf["C"])
	}
}

// TestTopologicalSort_Diamond verifies TopologicalSort handles a diamond
// dependency: D depends on B and C, both depend on A.
func TestTopologicalSort_Diamond(t *testing.T) {
	graph := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"A"},
		"D": {"B", "C"},
	}

	order, err := TopologicalSort(graph)
	if err != nil {
		t.Fatalf("TopologicalSort returned error for diamond: %v", err)
	}

	if len(order) != 4 {
		t.Fatalf("TopologicalSort returned %d items, want 4", len(order))
	}

	indexOf := make(map[string]int)
	for i, name := range order {
		indexOf[name] = i
	}

	// A before B and C, B and C before D
	if indexOf["A"] >= indexOf["B"] {
		t.Errorf("A must come before B")
	}
	if indexOf["A"] >= indexOf["C"] {
		t.Errorf("A must come before C")
	}
	if indexOf["B"] >= indexOf["D"] {
		t.Errorf("B must come before D")
	}
	if indexOf["C"] >= indexOf["D"] {
		t.Errorf("C must come before D")
	}
}

// TestTopologicalSort_NoDeps verifies TopologicalSort handles independent
// projects with no dependencies between them.
func TestTopologicalSort_NoDeps(t *testing.T) {
	graph := map[string][]string{
		"X": {},
		"Y": {},
		"Z": {},
	}

	order, err := TopologicalSort(graph)
	if err != nil {
		t.Fatalf("TopologicalSort returned error for independent projects: %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("TopologicalSort returned %d items, want 3", len(order))
	}

	// All three projects must appear (order is alphabetical due to deterministic sort)
	seen := make(map[string]bool)
	for _, name := range order {
		seen[name] = true
	}
	for _, name := range []string{"X", "Y", "Z"} {
		if !seen[name] {
			t.Errorf("TopologicalSort missing project %s", name)
		}
	}
}

// TestTopologicalSort_CycleDetection verifies TopologicalSort returns an
// error when a cycle exists in the dependency graph.
func TestTopologicalSort_CycleDetection(t *testing.T) {
	graph := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"A"},
	}

	_, err := TopologicalSort(graph)
	if err == nil {
		t.Fatal("TopologicalSort should return error for cyclic graph, got nil")
	}

	if !contains(err.Error(), "cycle") {
		t.Errorf("TopologicalSort error should mention 'cycle', got: %s", err.Error())
	}
}

// TestTopologicalSort_SingleNode verifies TopologicalSort handles a single
// project with no dependencies.
func TestTopologicalSort_SingleNode(t *testing.T) {
	graph := map[string][]string{
		"only": {},
	}

	order, err := TopologicalSort(graph)
	if err != nil {
		t.Fatalf("TopologicalSort returned error for single node: %v", err)
	}

	if len(order) != 1 || order[0] != "only" {
		t.Errorf("TopologicalSort returned %v, want [only]", order)
	}
}

// TestTopologicalSort_EmptyGraph verifies TopologicalSort handles an empty graph.
func TestTopologicalSort_EmptyGraph(t *testing.T) {
	graph := map[string][]string{}

	order, err := TopologicalSort(graph)
	if err != nil {
		t.Fatalf("TopologicalSort returned error for empty graph: %v", err)
	}

	if len(order) != 0 {
		t.Errorf("TopologicalSort returned %v for empty graph, want empty", order)
	}
}

// TestBuildDAG_WithSiblings verifies BuildDAG discovers sibling projects
// and builds correct dependency edges from vendor.yml files.
func TestBuildDAG_WithSiblings(t *testing.T) {
	// Create temp directory structure:
	//   root/
	//     project-a/   (no dependencies)
	//     project-b/   (depends on project-a via URL)
	root := t.TempDir()

	// project-a: has vendor.yml but no vendor dependencies
	mkVendorYML(t, root, "project-a", `
vendors:
  - name: external-lib
    url: https://github.com/someone/external-lib
    license: MIT
    specs:
      - ref: main
        mapping:
          - from: lib.go
            to: lib/lib.go
`)

	// project-b: vendors from project-a
	mkVendorYML(t, root, "project-b", `
vendors:
  - name: project-a
    url: https://github.com/myorg/project-a
    license: MIT
    specs:
      - ref: main
        mapping:
          - from: shared.go
            to: vendored/shared.go
`)

	svc := NewCascadeService(root)
	graph, dirs, err := svc.BuildDAG()
	if err != nil {
		t.Fatalf("BuildDAG returned error: %v", err)
	}

	// Both projects should be discovered
	if len(dirs) != 2 {
		t.Fatalf("BuildDAG found %d projects, want 2", len(dirs))
	}

	// project-b depends on project-a
	deps := graph["project-b"]
	if len(deps) != 1 || deps[0] != "project-a" {
		t.Errorf("project-b deps = %v, want [project-a]", deps)
	}

	// project-a has no sibling dependencies
	if len(graph["project-a"]) != 0 {
		t.Errorf("project-a deps = %v, want []", graph["project-a"])
	}
}

// TestBuildDAG_NoDirs verifies BuildDAG returns empty graph when root
// directory has no sibling projects.
func TestBuildDAG_NoDirs(t *testing.T) {
	root := t.TempDir()

	svc := NewCascadeService(root)
	graph, dirs, err := svc.BuildDAG()
	if err != nil {
		t.Fatalf("BuildDAG returned error: %v", err)
	}

	if len(graph) != 0 {
		t.Errorf("BuildDAG found %d projects, want 0", len(graph))
	}
	if len(dirs) != 0 {
		t.Errorf("BuildDAG found %d dirs, want 0", len(dirs))
	}
}

// TestCascade_DryRun verifies Cascade in dry-run mode returns the topological
// order without executing any pull operations.
func TestCascade_DryRun(t *testing.T) {
	root := t.TempDir()

	mkVendorYML(t, root, "alpha", `
vendors: []
`)

	mkVendorYML(t, root, "beta", `
vendors:
  - name: alpha
    url: https://github.com/myorg/alpha
    license: MIT
    specs:
      - ref: main
        mapping:
          - from: f.go
            to: v/f.go
`)

	svc := NewCascadeService(root)
	result, err := svc.Cascade(context.Background(), CascadeOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Cascade dry-run returned error: %v", err)
	}

	if len(result.Order) != 2 {
		t.Fatalf("Cascade order has %d items, want 2", len(result.Order))
	}

	// alpha must come before beta
	if result.Order[0] != "alpha" || result.Order[1] != "beta" {
		t.Errorf("Cascade order = %v, want [alpha, beta]", result.Order)
	}
}

// TestMatchSiblingByURL verifies the URL matching heuristic for identifying
// which sibling project a vendor URL references.
func TestMatchSiblingByURL(t *testing.T) {
	projects := map[string]bool{
		"git-plumbing": true,
		"git-agent":    true,
		"git-vendor":   true,
	}

	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/myorg/git-plumbing", "git-plumbing"},
		{"https://github.com/myorg/git-plumbing.git", "git-plumbing"},
		{"https://github.com/myorg/git-agent/", "git-agent"},
		{"https://example.com/unrelated-project", ""},
		{"../git-plumbing", "git-plumbing"},
		{"file://../git-vendor", "git-vendor"},
	}

	for _, tt := range tests {
		got := matchSiblingByURL(tt.url, projects)
		if got != tt.want {
			t.Errorf("matchSiblingByURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

// TestTopologicalSort_SelfCycle verifies TopologicalSort detects a self-referencing cycle.
func TestTopologicalSort_SelfCycle(t *testing.T) {
	graph := map[string][]string{
		"A": {"A"},
	}

	_, err := TopologicalSort(graph)
	if err == nil {
		t.Fatal("TopologicalSort should return error for self-cycle, got nil")
	}
}

// mkVendorYML creates a project directory with .git-vendor/vendor.yml
// at root/name/.git-vendor/vendor.yml with the given YAML content.
func mkVendorYML(t *testing.T, root, name, yamlContent string) {
	t.Helper()
	vendorDir := filepath.Join(root, name, VendorDir)
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("mkVendorYML: mkdir %s: %v", vendorDir, err)
	}
	path := filepath.Join(vendorDir, ConfigFile)
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("mkVendorYML: write %s: %v", path, err)
	}
}

