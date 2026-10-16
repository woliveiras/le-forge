package environment

import (
	"context"
	"strings"
	"testing"
)

func TestStageCopyValidationAndRewriting(t *testing.T) {
	for _, source := range []string{
		"FROM alpine AS build\nCOPY app /out\nFROM alpine AS runtime\nCOPY --from=build /out /app\n",
		"FROM alpine\nFROM alpine\nCOPY --from=0 [\"/path with spaces\",\"/app\"]\n",
		"FROM alpine AS runtime\nCOPY --from=build /out /app\nFROM alpine AS build\nRUN echo hi > /out\n",
	} {
		if err := validateRepositoryDockerfile(source); err != nil {
			t.Fatalf("valid stages: %v", err)
		}
		rewritten, err := contextBuildfile(source)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(rewritten, "context//out") || strings.Contains(rewritten, "context//path") {
			t.Fatalf("stage sources rewritten as repository paths: %s", rewritten)
		}
	}
	got, err := contextBuildfile("FROM alpine AS build\nCOPY app /out\nFROM alpine\nCOPY --from=build /out /app\n")
	if err != nil || got != "FROM alpine AS build\nCOPY [\"context/app\",\"/out\"]\nFROM alpine\nCOPY --from=build [\"/out\",\"/app\"]\n" {
		t.Fatalf("rewrite %q: %v", got, err)
	}
	for _, source := range []string{
		"FROM alpine AS build\nCOPY --from=build /out /app",
		"FROM alpine AS a\nCOPY --from=b /out /app\nFROM a AS b",
		"FROM alpine\nCOPY --from=9 /out /app",
		"FROM alpine\nCOPY --from=alpine:3.22 /out /app",
		"FROM alpine AS build\nFROM alpine\nCOPY --from=missing /out /app",
		"FROM alpine AS build\nFROM alpine\nADD --from=build /out /app",
		"FROM alpine AS build\nFROM alpine\nCOPY --from=build --chown=0 /out /app",
		"FROM alpine AS build\nFROM alpine\nCOPY --from=build ../outside /app",
		"FROM alpine AS build\nFROM alpine\nCOPY --from=build $FILE /app",
		"FROM alpine AS build\nFROM alpine\nCOPY --from= /out /app",
		"FROM alpine AS build\nFROM alpine\nCOPY --from=build",
		`FROM alpine AS build
FROM alpine
COPY --from=build ["", "/app"]`,
	} {
		if err := validateRepositoryDockerfile(source); err == nil {
			t.Fatalf("accepted invalid stages %s", source)
		}
	}
}

func TestResolveBuildTargets(t *testing.T) {
	for _, tc := range []struct{ path, body, service string }{
		{"Dockerfile", "", ""},
		{".leforge/environment.yaml", "dockerfile: build/Containerfile\ntarget: development", ""},
		{"compose.yaml", "services: {dev: {build: {context: build, dockerfile: Containerfile, target: development}}}", "dev"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			dir := t.TempDir()
			source := "FROM alpine AS development\nRUN echo dev\nFROM alpine AS runtime\nCOPY --from=development /etc/alpine-release /release\n"
			if tc.body == "" {
				writeDiscoveryFile(t, dir, tc.path, source)
			} else {
				writeDiscoveryFile(t, dir, tc.path, tc.body)
				writeDiscoveryFile(t, dir, "build/Containerfile", source)
			}
			sel := discoverSelection(t, dir, tc.service)
			if tc.body == "" {
				sel.Target = "development"
				discovery, err := Discover(context.Background(), dir)
				if err != nil || strings.Join(discovery.Candidates[0].Targets, ",") != "development,runtime" {
					t.Fatalf("targets: %+v %v", discovery, err)
				}
			}
			env, err := Resolve(context.Background(), dir, sel)
			if err != nil || env.Target != "development" {
				t.Fatalf("resolve %+v %v", env, err)
			}
			if err := ValidateAccepted(context.Background(), dir, env); err != nil {
				t.Fatal(err)
			}
			before := Hash(env)
			env.Target = "runtime"
			if Hash(env) == before {
				t.Fatal("target not part of image hash")
			}
			if err := ValidateAccepted(context.Background(), dir, env); err == nil {
				t.Fatal("accepted target tampering")
			}
			sel.Target = "missing"
			if _, err := Resolve(context.Background(), dir, sel); err == nil {
				t.Fatal("accepted invalid target selection")
			}
		})
	}
}
