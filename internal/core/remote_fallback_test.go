package core

import (
	"context"
	"errors"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

func TestResolveVendorURLs_ExternalWithMirrors(t *testing.T) {
	v := &types.VendorSpec{
		URL:     "https://github.com/org/repo",
		Mirrors: []string{"https://gitlab.com/org/repo", "https://internal.corp/repo"},
	}
	urls := ResolveVendorURLs(v)
	if len(urls) != 3 {
		t.Fatalf("Expected 3 URLs, got %d", len(urls))
	}
	if urls[0] != v.URL {
		t.Errorf("First URL should be primary, got %s", urls[0])
	}
	if urls[1] != v.Mirrors[0] {
		t.Errorf("Second URL should be first mirror, got %s", urls[1])
	}
}

func TestResolveVendorURLs_ExternalNoMirrors(t *testing.T) {
	v := &types.VendorSpec{URL: "https://github.com/org/repo"}
	urls := ResolveVendorURLs(v)
	if len(urls) != 1 {
		t.Fatalf("Expected 1 URL, got %d", len(urls))
	}
	if urls[0] != v.URL {
		t.Errorf("URL should be primary, got %s", urls[0])
	}
}

func TestResolveVendorURLs_InternalVendor(t *testing.T) {
	v := &types.VendorSpec{Source: SourceInternal}
	urls := ResolveVendorURLs(v)
	if urls != nil {
		t.Errorf("Internal vendors should return nil, got %v", urls)
	}
}

func TestFetchWithFallback_FirstURLSucceeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)

	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/repo", "origin", "https://primary.com/repo").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/repo", "origin", 1, "main").Return(nil)

	usedURL, err := FetchWithFallback(context.Background(), mockGit, mockFS, &SilentUICallback{},
		"/tmp/repo", []string{"https://primary.com/repo", "https://mirror.com/repo"}, "main", 1)

	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if usedURL != "https://primary.com/repo" {
		t.Errorf("Expected primary URL, got %s", usedURL)
	}
}

func TestFetchWithFallback_FallsBackToMirror(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)

	// Primary fails
	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/repo", "origin", "https://primary.com/repo").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/repo", "origin", 1, "main").Return(errors.New("connection refused"))

	// Switch to mirror and succeed
	mockGit.EXPECT().SetRemoteURL(gomock.Any(), "/tmp/repo", "origin", "https://mirror.com/repo").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/repo", "origin", 1, "main").Return(nil)

	usedURL, err := FetchWithFallback(context.Background(), mockGit, mockFS, &SilentUICallback{},
		"/tmp/repo", []string{"https://primary.com/repo", "https://mirror.com/repo"}, "main", 1)

	if err != nil {
		t.Fatalf("Expected success on mirror, got: %v", err)
	}
	if usedURL != "https://mirror.com/repo" {
		t.Errorf("Expected mirror URL, got %s", usedURL)
	}
}

func TestFetchWithFallback_AllURLsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)

	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/repo", "origin", "https://primary.com/repo").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/repo", "origin", 1, "main").Return(errors.New("timeout"))
	mockGit.EXPECT().SetRemoteURL(gomock.Any(), "/tmp/repo", "origin", "https://mirror.com/repo").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/repo", "origin", 1, "main").Return(errors.New("not found"))

	_, err := FetchWithFallback(context.Background(), mockGit, mockFS, &SilentUICallback{},
		"/tmp/repo", []string{"https://primary.com/repo", "https://mirror.com/repo"}, "main", 1)

	if err == nil {
		t.Fatal("Expected error when all URLs fail")
	}
	if !contains(err.Error(), "all URLs failed") {
		t.Errorf("Expected 'all URLs failed' error, got: %s", err.Error())
	}
}

func TestFetchWithFallback_EmptyURLs(t *testing.T) {
	_, err := FetchWithFallback(context.Background(), nil, nil, nil, "/tmp", nil, "main", 1)
	if err == nil {
		t.Fatal("Expected error for empty URLs")
	}
	if !contains(err.Error(), "no URLs provided") {
		t.Errorf("Expected 'no URLs provided' error, got: %s", err.Error())
	}
}

func TestFetchWithFallback_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := FetchWithFallback(ctx, nil, nil, nil, "/tmp",
		[]string{"https://example.com/repo"}, "main", 1)

	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
	if !contains(err.Error(), "cancelled") {
		t.Errorf("Expected error containing 'cancelled', got: %s", err.Error())
	}
}

func TestFetchWithFallback_SingleURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGitClient(ctrl)
	mockFS := NewMockFileSystem(ctrl)

	mockGit.EXPECT().AddRemote(gomock.Any(), "/tmp/repo", "origin", "https://only.com/repo").Return(nil)
	mockGit.EXPECT().Fetch(gomock.Any(), "/tmp/repo", "origin", 0, "v2").Return(nil)

	usedURL, err := FetchWithFallback(context.Background(), mockGit, mockFS, &SilentUICallback{},
		"/tmp/repo", []string{"https://only.com/repo"}, "v2", 0)

	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if usedURL != "https://only.com/repo" {
		t.Errorf("Expected only URL, got %s", usedURL)
	}
}
