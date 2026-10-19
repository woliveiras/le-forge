// Package localinstall owns the explicit, separate single-user native profile.
package localinstall

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/woliveiras/le-forge/server/internal/app"
	"github.com/woliveiras/le-forge/server/internal/config"
	"gopkg.in/yaml.v3"
)

const formatVersion = 1

type manifest struct {
	Version int `json:"version"`
}

func DefaultDir() (string, error) {
	d, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(d, "LeForge"), nil
}
func Profile(root, web string, port int) (*config.Config, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(web) || port < 1024 || port > 65535 {
		return nil, errors.New("native profile needs absolute data/UI paths and a port between 1024 and 65535")
	}
	c := config.Default()
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	url := "http://localhost:" + strconv.Itoa(port)
	c.Server.Addr = addr
	c.Server.PublicURL = url
	c.Server.InternalURL = "http://" + addr
	c.Server.WebDir = web
	c.Server.ProtectMetrics = true
	c.Preview = config.Preview{}
	c.Auth = config.Auth{Tokens: []config.Token{{Name: "local-admin", Role: config.RoleAdmin, TokenFile: filepath.Join(root, "admin.token")}}}
	c.Storage = config.Storage{Type: "sqlite", DSN: filepath.Join(root, "leforge.db")}
	c.Queue = config.Queue{Type: "fs", Dir: filepath.Join(root, "queues")}
	c.Workspaces = config.Workspaces{Root: filepath.Join(root, "workspaces")}
	c.Secrets = config.Secrets{KeyFile: filepath.Join(root, "secret.key"), GenerateKeyFile: true}
	c.Isolation.Docker.EnvironmentPolicy.SetupNetworks = []string{"none", "bridge"}
	c.Isolation.Docker.EnvironmentPolicy.RunNetworks = []string{"none", "bridge"}
	c.Instance = "leforge-native"
	return c, c.Validate()
}

// Init never overwrites an existing directory, even after partial initialization.
// No secret value is returned or logged.
func Init(root, web string, port int) error {
	c, err := Profile(root, web, port)
	if err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(web, "index.html")); err != nil {
		return fmt.Errorf("bundled web UI is missing: %w", err)
	}
	if err = os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return err
	}
	if err = os.Mkdir(root, 0700); err != nil {
		return fmt.Errorf("create native directory (existing data is never replaced): %w", err)
	}
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(root, "admin.token"), []byte(base64.RawURLEncoding.EncodeToString(token)+"\n"), 0600); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(root, "forge.yaml"), b, 0600); err != nil {
		return err
	}
	b, err = json.Marshal(manifest{Version: formatVersion})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "installation.json"), b, 0600)
}

// Open acquires a process lock before opening state. Native configuration does
// not expand or import LEFORGE_* environment values from another installation.
func Open(root string) (*config.Config, func(), error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize the native directory first: %w", err)
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, nil, errors.New("native directory must be a private directory (0700), not a symlink")
	}
	b, err := os.ReadFile(filepath.Join(root, "installation.json"))
	if err != nil {
		return nil, nil, err
	}
	var m manifest
	if json.Unmarshal(b, &m) != nil || m.Version != formatVersion {
		return nil, nil, errors.New("unsupported native installation format; data was not migrated")
	}
	f, err := os.OpenFile(filepath.Join(root, "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, nil, errors.New("this native data directory is already in use")
	}
	release := func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }
	fail := func(e error) (*config.Config, func(), error) { release(); return nil, nil, e }
	b, err = os.ReadFile(filepath.Join(root, "forge.yaml"))
	if err != nil {
		return fail(err)
	}
	c := config.Default()
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	decoder.KnownFields(true)
	if err = decoder.Decode(c); err != nil {
		return fail(err)
	}
	if err = c.Validate(); err != nil {
		return fail(err)
	}
	host, _, err := net.SplitHostPort(c.Server.Addr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || c.Auth.Disabled || len(c.Auth.Tokens) == 0 || c.Storage.Type != "sqlite" || c.Queue.Type != "fs" {
		return fail(errors.New("native profile requires loopback, authentication, SQLite and filesystem queues"))
	}
	for _, addr := range []string{c.Server.InternalAddr, c.Preview.Addr} {
		if addr == "" {
			continue
		}
		host, _, e := net.SplitHostPort(addr)
		if e != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			return fail(errors.New("native callback and preview listeners must use loopback"))
		}
	}
	if c.Storage.DSN != filepath.Join(root, "leforge.db") || c.Queue.Dir != filepath.Join(root, "queues") || c.Workspaces.Root != filepath.Join(root, "workspaces") || c.Secrets.KeyFile != filepath.Join(root, "secret.key") || c.Secrets.KeyEnv != "" || c.Secrets.PreviousKeysEnv != "" {
		return fail(errors.New("native state must stay in its own data directory without imported key environment variables"))
	}
	if _, err = os.Stat(filepath.Join(c.Server.WebDir, "index.html")); err != nil {
		return fail(errors.New("native web assets are unavailable"))
	}
	return c, release, nil
}
func Run(ctx context.Context, root string) error {
	c, release, err := Open(root)
	if err != nil {
		return err
	}
	defer release()
	a, err := app.New(ctx, c)
	if err != nil {
		return err
	}
	err = a.Run(ctx)
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		a.Engine.Shutdown(cleanup)
		return errors.Join(err, a.Close(cleanup))
	}
	return nil
}
