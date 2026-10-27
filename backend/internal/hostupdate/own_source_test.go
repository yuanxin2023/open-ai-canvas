package hostupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type releaseTransport func(*http.Request) (*http.Response, error)

func (f releaseTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func completeTestRelease(version string) githubRelease {
	assets := []githubReleaseAsset{}
	for _, name := range []string{"SHA256SUMS", "open-ai-canvas-host-updater-linux-amd64", "open-ai-canvas-host-updater-linux-arm64", "docker-compose.deploy.yml"} {
		assets = append(assets, githubReleaseAsset{Name: name, State: "uploaded", Size: 100})
	}
	return githubRelease{TagName: version, Assets: assets}
}

func TestOwnSourceAcceptsOnlyStableCompleteReleases(t *testing.T) {
	m := &Manager{config: Config{ComposeFile: "docker-compose.deploy.yml"}}
	for _, version := range []string{"v1.2.3", "v1.6.9.2", "v1.6.9.2.1"} {
		if !m.completeRelease(completeTestRelease(version)) {
			t.Fatalf("stable rejected: %s", version)
		}
	}
	for _, version := range []string{"v1.2.3-rc.1", "v1.2.3-", "v1.2.3+build", "main", "latest", "v1.2", "v1.2.-3", "v+1.2.3"} {
		if m.completeRelease(completeTestRelease(version)) {
			t.Fatalf("non-stable accepted: %s", version)
		}
	}
	for _, mutate := range []func(*githubRelease){
		func(r *githubRelease) { r.Draft = true }, func(r *githubRelease) { r.Prerelease = true },
		func(r *githubRelease) { r.Assets = r.Assets[1:] }, func(r *githubRelease) { r.Assets[0].Size = 0 }, func(r *githubRelease) { r.Assets[0].State = "open" },
	} {
		release := completeTestRelease("v1.2.3")
		mutate(&release)
		if m.completeRelease(release) {
			t.Fatalf("incomplete accepted: %#v", release)
		}
	}
}

func TestOwnSourceLatestReleaseUsesConfiguredRepoAndSkipsIncompleteNewerRelease(t *testing.T) {
	newer := completeTestRelease("v9.0.0")
	newer.Assets = nil
	preview := completeTestRelease("v10.0.0")
	preview.Prerelease = true
	data, _ := json.Marshal([]githubRelease{newer, completeTestRelease("v1.2.3"), completeTestRelease("v1.2.3.1"), preview})
	m := &Manager{config: Config{Repository: DefaultRepository, ComposeFile: "docker-compose.deploy.yml"}, httpClient: &http.Client{Transport: releaseTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.github.com/repos/yuanxin2023/open-ai-canvas/releases?per_page=30" {
			t.Fatalf("wrong source: %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}, nil
	})}}
	got, err := m.latestRelease(context.Background())
	if err != nil || got.Version != "v1.2.3.1" {
		t.Fatalf("latest complete stable: %#v %v", got, err)
	}
}

func TestOwnSourceClearsForeignCachedReleaseWithoutDiscardingBackup(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(persistedState{Repository: "ddcat-ai/open-ai-canvas", LatestRelease: &Release{Version: "v9.0.0"}, LastBackup: &Backup{ID: "preserve"}, Operation: Operation{Phase: PhaseReady}})
	if err := os.WriteFile(filepath.Join(state, "state.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(Config{InstallDir: root, StateDir: state, BackupDir: filepath.Join(root, "backups")})
	if err != nil {
		t.Fatal(err)
	}
	if m.config.Repository != DefaultRepository || m.state.LatestRelease != nil || m.state.LastBackup == nil || m.state.LastBackup.ID != "preserve" {
		t.Fatalf("wrong source transition: %#v", m.state)
	}
}

type imageMetadataRunner struct {
	mutate func(int, map[string]any)
	calls  int
}

func (r *imageMetadataRunner) Run(_ context.Context, name string, args, _ []string, out, _ io.Writer) error {
	if name != "docker" || len(args) < 3 || args[0] != "image" || args[1] != "inspect" {
		return fmt.Errorf("unexpected command: %s %v", name, args)
	}
	repository, _, _ := strings.Cut(args[2], ":")
	value := map[string]any{"RepoDigests": []string{repository + "@sha256:" + strings.Repeat("a", 64)}, "Config": map[string]any{"Labels": map[string]string{"org.opencontainers.image.version": "v1.2.3", "org.opencontainers.image.source": "https://github.com/" + DefaultRepository, "org.opencontainers.image.revision": strings.Repeat("b", 40)}}}
	if r.mutate != nil {
		r.mutate(r.calls, value)
	}
	r.calls++
	return json.NewEncoder(out).Encode(value)
}

func TestOwnSourceImageVerificationRejectsMixedBuilds(t *testing.T) {
	for _, field := range []string{"", "version", "source", "revision", "mixed", "digest"} {
		t.Run(field, func(t *testing.T) {
			runner := &imageMetadataRunner{mutate: func(index int, value map[string]any) {
				labels := value["Config"].(map[string]any)["Labels"].(map[string]string)
				switch field {
				case "version":
					labels["org.opencontainers.image.version"] = "v1.2.4"
				case "source":
					labels["org.opencontainers.image.source"] = "https://github.com/ddcat-ai/open-ai-canvas"
				case "revision":
					labels["org.opencontainers.image.revision"] = "unknown"
				case "mixed":
					if index == 1 {
						labels["org.opencontainers.image.revision"] = strings.Repeat("c", 40)
					}
				case "digest":
					value["RepoDigests"] = []string{"ghcr.io/other/image@sha256:" + strings.Repeat("a", 64)}
				}
			}}
			m := &Manager{config: Config{Repository: DefaultRepository}, runner: runner}
			images, err := m.verifyImages("v1.2.3")
			if field == "" {
				if err != nil || !strings.HasPrefix(images.agent, "ghcr.io/yuanxin2023/open-ai-canvas-yingce-agent@sha256:") || runner.calls != 3 {
					t.Fatalf("matching images: %#v %v", images, err)
				}
			} else if err == nil {
				t.Fatal("mismatched image accepted")
			}
		})
	}
}

func TestOwnSourceStartRequiresFreshStableCheck(t *testing.T) {
	for _, phase := range []Phase{PhaseFailed, PhaseChecking, PhaseIdle} {
		m := &Manager{state: persistedState{LatestRelease: &Release{Version: "v1.2.3"}, Operation: Operation{Phase: phase}}}
		if _, err := m.StartUpdate("v1.2.3"); err == nil {
			t.Fatalf("start accepted in %s", phase)
		}
	}
	m := &Manager{state: persistedState{LatestRelease: &Release{Version: "v1.2.3-rc.1", Prerelease: true}, Operation: Operation{Phase: PhaseReady}}}
	if _, err := m.StartUpdate("v1.2.3-rc.1"); err == nil {
		t.Fatal("prerelease start accepted")
	}
}
