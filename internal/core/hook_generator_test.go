package core

import (
	"strings"
	"testing"
)

func TestGeneratePreCommitHook_ContainsShebang(t *testing.T) {
	hook := GeneratePreCommitHook()
	if !strings.HasPrefix(hook, "#!/bin/sh\n") {
		t.Error("GeneratePreCommitHook must start with #!/bin/sh shebang")
	}
}

func TestGeneratePreCommitHook_ContainsBinaryGuard(t *testing.T) {
	hook := GeneratePreCommitHook()
	if !strings.Contains(hook, "command -v git-vendor") {
		t.Error("GeneratePreCommitHook must guard against missing git-vendor binary")
	}
}

func TestGeneratePreCommitHook_ContainsStatusCall(t *testing.T) {
	hook := GeneratePreCommitHook()
	if !strings.Contains(hook, "git-vendor status --offline") {
		t.Error("GeneratePreCommitHook must call git-vendor status --offline")
	}
}

func TestGeneratePreCommitHook_DifferentiatesStrictAndLenient(t *testing.T) {
	hook := GeneratePreCommitHook()
	// Exit 0 passes, exit 2 (lenient) warns but passes, exit 1 (strict) blocks.
	if !strings.Contains(hook, `"$EXIT_CODE" -eq 0`) {
		t.Error("GeneratePreCommitHook must check for exit code 0 to pass")
	}
	if !strings.Contains(hook, `"$EXIT_CODE" -eq 2`) {
		t.Error("GeneratePreCommitHook must check for exit code 2 (lenient) separately")
	}
	if !strings.Contains(hook, "warning only") {
		t.Error("GeneratePreCommitHook must label lenient drift as warning only")
	}
	if !strings.Contains(hook, "Commit blocked.") {
		t.Error("GeneratePreCommitHook must block on strict drift (exit 1)")
	}
}

func TestGenerateMakefileTarget_ContainsStrictOnly(t *testing.T) {
	target := GenerateMakefileTarget()
	if !strings.Contains(target, "--strict-only") {
		t.Error("GenerateMakefileTarget must use --strict-only flag")
	}
}

func TestGenerateMakefileTarget_IsPhony(t *testing.T) {
	target := GenerateMakefileTarget()
	if !strings.Contains(target, ".PHONY: vendor-check") {
		t.Error("GenerateMakefileTarget must declare vendor-check as .PHONY")
	}
}
