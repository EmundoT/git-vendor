package providers

import (
	"testing"
)

// ============================================================================
// BitbucketProvider Tests
// ============================================================================

func TestBitbucketProvider_Name(t *testing.T) {
	provider := NewBitbucketProvider()
	if provider.Name() != "bitbucket" {
		t.Errorf("Expected 'bitbucket', got '%s'", provider.Name())
	}
}

func TestBitbucketProvider_Supports(t *testing.T) {
	provider := NewBitbucketProvider()

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"bitbucket.org URL", "https://bitbucket.org/owner/repo", true},
		{"bitbucket without protocol", "bitbucket.org/owner/repo", true},
		{"bitbucket deep link", "https://bitbucket.org/owner/repo/src/main/file.py", true},
		{"github URL", "https://github.com/owner/repo", false},
		{"gitlab URL", "https://gitlab.com/owner/repo", false},
		{"generic URL", "https://git.example.com/repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.Supports(tt.url)
			if result != tt.expected {
				t.Errorf("Supports(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestBitbucketProvider_ParseURL(t *testing.T) {
	provider := NewBitbucketProvider()

	tests := []struct {
		name      string
		url       string
		wantBase  string
		wantRef   string
		wantPath  string
		wantError bool
	}{
		{
			name:      "basic URL with https",
			url:       "https://bitbucket.org/owner/repo",
			wantBase:  "https://bitbucket.org/owner/repo",
			wantRef:   "",
			wantPath:  "",
			wantError: false,
		},
		{
			name:      "basic URL without protocol",
			url:       "bitbucket.org/owner/repo",
			wantBase:  "https://bitbucket.org/owner/repo",
			wantRef:   "",
			wantPath:  "",
			wantError: false,
		},
		{
			name:      "basic URL with .git suffix",
			url:       "https://bitbucket.org/owner/repo.git",
			wantBase:  "https://bitbucket.org/owner/repo",
			wantRef:   "",
			wantPath:  "",
			wantError: false,
		},
		{
			name:      "deep link with main branch",
			url:       "https://bitbucket.org/owner/repo/src/main/path/to/file.py",
			wantBase:  "https://bitbucket.org/owner/repo",
			wantRef:   "main",
			wantPath:  "path/to/file.py",
			wantError: false,
		},
		{
			name:      "deep link with version tag",
			url:       "https://bitbucket.org/owner/repo/src/v1.0.0/src/components/Button.jsx",
			wantBase:  "https://bitbucket.org/owner/repo",
			wantRef:   "v1.0.0",
			wantPath:  "src/components/Button.jsx",
			wantError: false,
		},
		{
			name:      "deep link with develop branch",
			url:       "bitbucket.org/team/project/src/develop/lib/util.js",
			wantBase:  "https://bitbucket.org/team/project",
			wantRef:   "develop",
			wantPath:  "lib/util.js",
			wantError: false,
		},
		{
			name:      "invalid URL - no owner/repo",
			url:       "https://bitbucket.org",
			wantError: true,
		},
		{
			name:      "invalid URL - incomplete path",
			url:       "https://bitbucket.org/owner",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path, err := provider.ParseURL(tt.url)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for URL %q, got nil", tt.url)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if base != tt.wantBase {
				t.Errorf("ParseURL(%q) base = %q, want %q", tt.url, base, tt.wantBase)
			}
			if ref != tt.wantRef {
				t.Errorf("ParseURL(%q) ref = %q, want %q", tt.url, ref, tt.wantRef)
			}
			if path != tt.wantPath {
				t.Errorf("ParseURL(%q) path = %q, want %q", tt.url, path, tt.wantPath)
			}
		})
	}
}

// ============================================================================
// GitLabProvider Tests
// ============================================================================

func TestGitLabProvider_Name(t *testing.T) {
	provider := NewGitLabProvider()
	if provider.Name() != "gitlab" {
		t.Errorf("Expected 'gitlab', got '%s'", provider.Name())
	}
}

func TestGitLabProvider_Supports(t *testing.T) {
	provider := NewGitLabProvider()

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"gitlab.com URL", "https://gitlab.com/owner/repo", true},
		{"gitlab without protocol", "gitlab.com/owner/repo", true},
		{"gitlab blob link", "https://gitlab.com/owner/repo/-/blob/main/file.go", true},
		{"gitlab tree link", "https://gitlab.com/owner/repo/-/tree/main/src/", true},
		{"self-hosted gitlab with /-/blob/", "https://gitlab.example.com/team/project/-/blob/dev/README.md", true},
		{"self-hosted gitlab with /-/tree/", "https://git.company.com/group/subgroup/repo/-/tree/master/docs/", true},
		{"github URL", "https://github.com/owner/repo", false},
		{"bitbucket URL", "https://bitbucket.org/owner/repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.Supports(tt.url)
			if result != tt.expected {
				t.Errorf("Supports(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestGitLabProvider_ParseURL(t *testing.T) {
	provider := NewGitLabProvider()

	tests := []struct {
		name      string
		url       string
		wantBase  string
		wantRef   string
		wantPath  string
		wantError bool
	}{
		{
			name:      "basic gitlab.com URL",
			url:       "https://gitlab.com/owner/repo",
			wantBase:  "https://gitlab.com/owner/repo",
			wantRef:   "",
			wantPath:  "",
			wantError: false,
		},
		{
			name:      "gitlab.com URL without protocol",
			url:       "gitlab.com/owner/repo",
			wantBase:  "https://gitlab.com/owner/repo",
			wantRef:   "",
			wantPath:  "",
			wantError: false,
		},
		{
			name:      "gitlab.com URL with .git suffix",
			url:       "https://gitlab.com/owner/repo.git",
			wantBase:  "https://gitlab.com/owner/repo",
			wantRef:   "",
			wantPath:  "",
			wantError: false,
		},
		{
			name:      "gitlab blob link with main branch",
			url:       "https://gitlab.com/owner/repo/-/blob/main/path/to/file.go",
			wantBase:  "https://gitlab.com/owner/repo",
			wantRef:   "main",
			wantPath:  "path/to/file.go",
			wantError: false,
		},
		{
			name:      "gitlab tree link with tag",
			url:       "https://gitlab.com/owner/repo/-/tree/v1.0.0/src/components/",
			wantBase:  "https://gitlab.com/owner/repo",
			wantRef:   "v1.0.0",
			wantPath:  "src/components/",
			wantError: false,
		},
		{
			name:      "nested groups",
			url:       "https://gitlab.com/owner/group/subgroup/repo/-/blob/develop/lib/util.js",
			wantBase:  "https://gitlab.com/owner/group/subgroup/repo",
			wantRef:   "develop",
			wantPath:  "lib/util.js",
			wantError: false,
		},
		{
			name:      "self-hosted gitlab",
			url:       "https://gitlab.example.com/team/project/-/blob/master/README.md",
			wantBase:  "https://gitlab.example.com/team/project",
			wantRef:   "master",
			wantPath:  "README.md",
			wantError: false,
		},
		{
			name:      "self-hosted without protocol",
			url:       "gitlab.company.com/engineering/backend/api",
			wantBase:  "https://gitlab.company.com/engineering/backend/api",
			wantRef:   "",
			wantPath:  "",
			wantError: false,
		},
		{
			name:      "gitlab with trailing slash",
			url:       "https://gitlab.com/owner/repo/",
			wantBase:  "https://gitlab.com/owner/repo",
			wantRef:   "",
			wantPath:  "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path, err := provider.ParseURL(tt.url)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for URL %q, got nil", tt.url)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if base != tt.wantBase {
				t.Errorf("ParseURL(%q) base = %q, want %q", tt.url, base, tt.wantBase)
			}
			if ref != tt.wantRef {
				t.Errorf("ParseURL(%q) ref = %q, want %q", tt.url, ref, tt.wantRef)
			}
			if path != tt.wantPath {
				t.Errorf("ParseURL(%q) path = %q, want %q", tt.url, path, tt.wantPath)
			}
		})
	}
}

// ============================================================================
// GenericProvider Tests
// ============================================================================

func TestGenericProvider_Name(t *testing.T) {
	provider := NewGenericProvider()
	if provider.Name() != "generic" {
		t.Errorf("Expected 'generic', got '%s'", provider.Name())
	}
}

func TestGenericProvider_Supports(t *testing.T) {
	provider := NewGenericProvider()

	tests := []struct {
		name string
		url  string
	}{
		{"github URL", "https://github.com/owner/repo"},
		{"gitlab URL", "https://gitlab.com/owner/repo"},
		{"bitbucket URL", "https://bitbucket.org/owner/repo"},
		{"custom git server", "https://git.example.com/project/repo"},
		{"git protocol", "git://git.kernel.org/pub/scm/git/git.git"},
		{"ssh URL", "git@github.com:owner/repo.git"},
		{"random URL", "https://example.com/something"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.Supports(tt.url)
			if !result {
				t.Errorf("GenericProvider.Supports(%q) = false, want true (generic provider should accept all URLs)", tt.url)
			}
		})
	}
}

func TestGenericProvider_ParseURL(t *testing.T) {
	provider := NewGenericProvider()

	tests := []struct {
		name     string
		url      string
		wantBase string
	}{
		{
			name:     "https URL",
			url:      "https://git.example.com/project/repo",
			wantBase: "https://git.example.com/project/repo",
		},
		{
			name:     "http URL",
			url:      "http://git.example.com/project/repo",
			wantBase: "http://git.example.com/project/repo",
		},
		{
			name:     "URL without protocol",
			url:      "git.example.com/project/repo",
			wantBase: "https://git.example.com/project/repo",
		},
		{
			name:     "git protocol",
			url:      "git://git.kernel.org/pub/scm/git/git.git",
			wantBase: "git://git.kernel.org/pub/scm/git/git",
		},
		{
			name:     "ssh URL",
			url:      "git@github.com:owner/repo.git",
			wantBase: "git@github.com:owner/repo",
		},
		{
			name:     "ssh URL with ssh protocol",
			url:      "ssh://git@server.com/path/to/repo.git",
			wantBase: "ssh://git@server.com/path/to/repo",
		},
		{
			name:     "URL with .git suffix",
			url:      "https://git.company.com/team/project.git",
			wantBase: "https://git.company.com/team/project",
		},
		{
			name:     "URL with trailing slash",
			url:      "https://git.example.com/repo/",
			wantBase: "https://git.example.com/repo",
		},
		{
			name:     "URL with trailing slash and .git",
			url:      "https://git.example.com/repo.git/",
			wantBase: "https://git.example.com/repo.git", // Trailing slash removed first, so .git remains
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path, err := provider.ParseURL(tt.url)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if base != tt.wantBase {
				t.Errorf("ParseURL(%q) base = %q, want %q", tt.url, base, tt.wantBase)
			}

			// Generic provider should always return empty ref and path
			if ref != "" {
				t.Errorf("ParseURL(%q) ref = %q, want empty string", tt.url, ref)
			}
			if path != "" {
				t.Errorf("ParseURL(%q) path = %q, want empty string", tt.url, path)
			}
		})
	}
}

// ============================================================================
// GitHubProvider Tests (Edge Cases)
// ============================================================================

func TestGitHubProvider_ParseURL_EdgeCases(t *testing.T) {
	provider := NewGitHubProvider()

	tests := []struct {
		name     string
		url      string
		wantBase string
		wantRef  string
		wantPath string
	}{
		{
			name:     "URL with leading backslash",
			url:      "\\github.com/owner/repo",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "URL with leading whitespace",
			url:      "  https://github.com/owner/repo  ",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "",
			wantPath: "",
		},
		{
			name:     "blob link with special characters in path",
			url:      "https://github.com/owner/repo/blob/main/path/with-dashes_underscores.file.ext",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "main",
			wantPath: "path/with-dashes_underscores.file.ext",
		},
		{
			name:     "tree link with nested directories",
			url:      "https://github.com/owner/repo/tree/develop/src/components/ui/buttons/",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "develop",
			wantPath: "src/components/ui/buttons/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path, err := provider.ParseURL(tt.url)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if base != tt.wantBase {
				t.Errorf("ParseURL(%q) base = %q, want %q", tt.url, base, tt.wantBase)
			}
			if ref != tt.wantRef {
				t.Errorf("ParseURL(%q) ref = %q, want %q", tt.url, ref, tt.wantRef)
			}
			if path != tt.wantPath {
				t.Errorf("ParseURL(%q) path = %q, want %q", tt.url, path, tt.wantPath)
			}
		})
	}
}

// ============================================================================
// ProviderRegistry Tests
// ============================================================================

func TestProviderRegistry_DetectProvider(t *testing.T) {
	registry := NewProviderRegistry()

	tests := []struct {
		name         string
		url          string
		wantProvider string
	}{
		{"github.com URL", "https://github.com/owner/repo", "github"},
		{"github.com blob link", "https://github.com/owner/repo/blob/main/file.go", "github"},
		{"gitlab.com URL", "https://gitlab.com/owner/repo", "gitlab"},
		{"gitlab.com blob link", "https://gitlab.com/owner/repo/-/blob/main/file.go", "gitlab"},
		{"self-hosted gitlab with /-/blob/", "https://git.example.com/owner/repo/-/blob/main/file.go", "gitlab"},
		{"bitbucket.org URL", "https://bitbucket.org/owner/repo", "bitbucket"},
		{"bitbucket.org src link", "https://bitbucket.org/owner/repo/src/main/file.py", "bitbucket"},
		{"custom git server", "https://git.company.com/project/repo", "generic"},
		{"git protocol", "git://git.kernel.org/pub/scm/git/git.git", "generic"},
		{"ssh URL", "git@server.com:owner/repo.git", "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := registry.DetectProvider(tt.url)
			if provider.Name() != tt.wantProvider {
				t.Errorf("DetectProvider(%q) = %q, want %q", tt.url, provider.Name(), tt.wantProvider)
			}
		})
	}
}

func TestProviderRegistry_ParseURL(t *testing.T) {
	registry := NewProviderRegistry()

	tests := []struct {
		name     string
		url      string
		wantBase string
		wantRef  string
		wantPath string
	}{
		{
			name:     "github deep link",
			url:      "https://github.com/owner/repo/blob/main/src/file.go",
			wantBase: "https://github.com/owner/repo",
			wantRef:  "main",
			wantPath: "src/file.go",
		},
		{
			name:     "gitlab deep link",
			url:      "https://gitlab.com/owner/repo/-/blob/main/src/file.go",
			wantBase: "https://gitlab.com/owner/repo",
			wantRef:  "main",
			wantPath: "src/file.go",
		},
		{
			name:     "bitbucket deep link",
			url:      "https://bitbucket.org/owner/repo/src/main/file.py",
			wantBase: "https://bitbucket.org/owner/repo",
			wantRef:  "main",
			wantPath: "file.py",
		},
		{
			name:     "generic git URL",
			url:      "https://git.example.com/project/repo",
			wantBase: "https://git.example.com/project/repo",
			wantRef:  "",
			wantPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ref, path, err := registry.ParseURL(tt.url)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if base != tt.wantBase {
				t.Errorf("ParseURL(%q) base = %q, want %q", tt.url, base, tt.wantBase)
			}
			if ref != tt.wantRef {
				t.Errorf("ParseURL(%q) ref = %q, want %q", tt.url, ref, tt.wantRef)
			}
			if path != tt.wantPath {
				t.Errorf("ParseURL(%q) path = %q, want %q", tt.url, path, tt.wantPath)
			}
		})
	}
}

// ============================================================================
// cleanURL Helper Tests
// ============================================================================

func TestCleanURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal URL", "https://github.com/owner/repo", "https://github.com/owner/repo"},
		{"URL with leading whitespace", "  https://github.com/owner/repo", "https://github.com/owner/repo"},
		{"URL with trailing whitespace", "https://github.com/owner/repo  ", "https://github.com/owner/repo"},
		{"URL with leading backslash", "\\github.com/owner/repo", "github.com/owner/repo"},
		{"URL with both whitespace and backslash", " \\ https://github.com/owner/repo ", " https://github.com/owner/repo"}, // Space after backslash remains
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanURL(tt.input)
			if result != tt.expected {
				t.Errorf("cleanURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
