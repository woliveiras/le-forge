package graph

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/woliveiras/le-forge/server/internal/domain"
)

// MaxEnvironmentTimeout bounds environment builds.
const MaxEnvironmentTimeout = 2 * time.Hour

// Directories a cache volume may not cover: they would hide the workspace
// or the system.
var reservedCacheDirs = []string{"/", "/workspace", "/bin", "/sbin", "/usr", "/lib", "/lib64", "/etc", "/proc", "/sys", "/dev", "/var", "/tmp", "/root", "/home"}

func validateEnvironment(r *Report, env *domain.EnvironmentSpec) {
	const p = "spec.environment"
	image, dockerfile := strings.TrimSpace(env.Image), strings.TrimSpace(env.Dockerfile)
	switch {
	case image == "" && dockerfile == "":
		r.errf(p, "set image or dockerfile")
	case image != "" && dockerfile != "":
		r.errf(p, "set either image or dockerfile, not both")
	case image != "" && (strings.ContainsAny(image, " \t\n") || strings.HasPrefix(image, "-")):
		r.errf(p+".image", "%q is not an image reference", env.Image)
	case dockerfile != "" && !hasFromLine(dockerfile):
		r.errf(p+".dockerfile", "a Dockerfile needs a FROM instruction")
	}
	for i, cmd := range env.Setup {
		if strings.TrimSpace(cmd) == "" {
			r.errf(fmt.Sprintf("%s.setup[%d]", p, i), "empty setup command")
		}
	}
	for field, v := range map[string]string{"network.setup": env.Network.Setup, "network.run": env.Network.Run} {
		if v != "" && v != "none" && v != "bridge" {
			r.errf(p+"."+field, "must be none or bridge")
		}
	}
	seen := map[string]bool{}
	for i, dir := range env.Caches {
		path_ := fmt.Sprintf("%s.caches[%d]", p, i)
		clean := path.Clean(dir)
		switch {
		case !path.IsAbs(dir):
			r.errf(path_, "%q must be an absolute directory", dir)
		case isReservedCacheDir(clean):
			r.errf(path_, "%q would hide the workspace or system files; use a directory such as /home/sandbox/.cache", dir)
		case seen[clean]:
			r.errf(path_, "duplicate cache %q", dir)
		}
		seen[clean] = true
	}
	if env.Timeout < 0 || env.Timeout.D() > MaxEnvironmentTimeout {
		r.errf(p+".timeout", "must be between 0 (default 30m) and %s", MaxEnvironmentTimeout)
	}
}

func hasFromLine(dockerfile string) bool {
	for _, line := range strings.Split(dockerfile, "\n") {
		if f := strings.Fields(line); len(f) > 1 && strings.EqualFold(f[0], "FROM") {
			return true
		}
	}
	return false
}

func isReservedCacheDir(dir string) bool {
	for _, r := range reservedCacheDirs {
		if dir == r {
			return true
		}
	}
	return strings.HasPrefix(dir, "/workspace/") || strings.HasPrefix(dir, "/proc/") || strings.HasPrefix(dir, "/sys/") || strings.HasPrefix(dir, "/dev/")
}

// ValidateEnvironment exposes the same checks used when saving graphs.
func ValidateEnvironment(env *domain.EnvironmentSpec) *Report {
	r := &Report{}
	if env == nil {
		r.errf("spec.environment", "set an environment")
	} else {
		validateEnvironment(r, env)
	}
	r.Valid = len(r.Errors) == 0
	return r
}
