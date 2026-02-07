package daemon

import (
	"strings"
	"testing"
)

func TestSocketPathDifferentConfigPaths(t *testing.T) {
	t.Setenv("COMPROC_SOCKET", "")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	path1 := SocketPath("/home/user/project-a/comproc.yaml")
	path2 := SocketPath("/home/user/project-b/comproc.yaml")

	if path1 == path2 {
		t.Errorf("different config paths should produce different socket paths, got %s for both", path1)
	}
}

func TestSocketPathSameConfigPath(t *testing.T) {
	t.Setenv("COMPROC_SOCKET", "")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	path1 := SocketPath("/home/user/project/comproc.yaml")
	path2 := SocketPath("/home/user/project/comproc.yaml")

	if path1 != path2 {
		t.Errorf("same config path should produce same socket path, got %s and %s", path1, path2)
	}
}

func TestSocketPathEnvOverride(t *testing.T) {
	t.Setenv("COMPROC_SOCKET", "/custom/path.sock")

	path := SocketPath("/any/config/path.yaml")

	if path != "/custom/path.sock" {
		t.Errorf("COMPROC_SOCKET should override, got %s", path)
	}
}

func TestSocketPathUsesXDGRuntimeDir(t *testing.T) {
	t.Setenv("COMPROC_SOCKET", "")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	path := SocketPath("/home/user/project/comproc.yaml")

	if !strings.HasPrefix(path, "/run/user/1000/") {
		t.Errorf("should use XDG_RUNTIME_DIR, got %s", path)
	}
	if !strings.HasPrefix(path, "/run/user/1000/comproc-") || !strings.HasSuffix(path, ".sock") {
		t.Errorf("should match pattern comproc-{hash}.sock, got %s", path)
	}
}
