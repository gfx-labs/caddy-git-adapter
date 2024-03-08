package gitadapter

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

func init() {
	caddyconfig.RegisterAdapter("git", Adapter{})
}

type GitAdapterConfig struct {
	Url       string                 `json:"url"`
	Ref       plumbing.ReferenceName `json:"ref"`
	ClonePath string                 `json:"clone_path"`
	Caddyfile string                 `json:"caddyfile"`
}

type Adapter struct{}

var tmpDir = os.TempDir()

func (a Adapter) Adapt(body []byte, options map[string]interface{}) (
	[]byte, []caddyconfig.Warning, error) {

	adapterConfig := GitAdapterConfig{}

	err := yaml.Unmarshal(body, &adapterConfig)
	if err != nil {
		return nil, nil, err
	}

	if adapterConfig.Url == "" {
		caddy.Log().Named("adapters.git.config").Error(fmt.Sprintf("url Not Found"))
		panic("CaddyGitAdapter url Not Found")
	}

	if adapterConfig.Ref == "" {
		adapterConfig.Ref = plumbing.Master
	}
	if adapterConfig.ClonePath == "" {
		adapterConfig.ClonePath = tmpDir
	}
	if adapterConfig.Caddyfile == "" {
		adapterConfig.Caddyfile = "Caddyfile"
	}
	p, err := url.Parse(adapterConfig.Url)
	if err != nil {
		return nil, nil, err
	}

	repoClonePath := path.Join(adapterConfig.ClonePath, p.Host, p.Path)
	os.MkdirAll(repoClonePath, 0o666)

	caddy.Log().Named("adapters.git.config").Info("cloning to", zap.String("dir", repoClonePath))
	r, err := git.PlainClone(repoClonePath, false, &git.CloneOptions{
		URL:           adapterConfig.Url,
		ReferenceName: adapterConfig.Ref,
	})
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		r, err = git.PlainOpen(repoClonePath)
		if err != nil {
			return nil, nil, errors.New("directory already exists and is not git repository")
		}
	} else if err != nil {
		return nil, nil, err
	}
	err = r.Fetch(&git.FetchOptions{
		Force: true,
	})
	if err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil, nil, err
		}
	}
	workTree, err := r.Worktree()
	if err != nil {
		return nil, nil, err
	}

	err = workTree.Reset(&git.ResetOptions{
		Mode: git.HardReset,
	})
	if err != nil {
		return nil, nil, err
	}
	err = workTree.Clean(&git.CleanOptions{
		Dir: true,
	})
	if err != nil {
		return nil, nil, err
	}
	err = workTree.Checkout(&git.CheckoutOptions{
		Branch: adapterConfig.Ref,
	})
	if err != nil {
		return nil, nil, err
	}
	err = workTree.Pull(&git.PullOptions{
		Force:         true,
		ReferenceName: adapterConfig.Ref,
	})
	if err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil, nil, err
		}
	}

	config, _, err := caddycmd.LoadConfig(path.Join(repoClonePath, adapterConfig.Caddyfile), "caddyfile")
	if err != nil {
		return nil, nil, err
	}

	return config, nil, err
}

var _ caddyconfig.Adapter = (*Adapter)(nil)
