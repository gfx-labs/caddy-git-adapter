package gitadapter

import (
	"errors"
	"fmt"
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
	Ref       plumbing.ReferenceName `json:"branch"`
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
		panic("CaddyGitAdapter Dsn Not Found")
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
	caddy.Log().Named("adapters.git.config").Info("cloning to", zap.String("dir", adapterConfig.ClonePath))
	// clone the files in head into memory, with 0 depth
	r, err := git.PlainClone(adapterConfig.ClonePath, false, &git.CloneOptions{
		URL:           adapterConfig.Url,
		Depth:         0,
		SingleBranch:  true,
		ReferenceName: adapterConfig.Ref,
	})
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		r, err = git.PlainOpen(adapterConfig.ClonePath)
		if err != nil {
			return nil, nil, errors.New("directory already exists and is not git repository")
		}
	}
	workTree, err := r.Worktree()
	if err != nil {
		return nil, nil, err
	}

	err = workTree.Clean(&git.CleanOptions{
		Dir: true,
	})
	if err != nil {
		return nil, nil, err
	}
	commit, err := r.ResolveRevision(plumbing.Revision(adapterConfig.Ref))
	if err != nil {
		return nil, nil, err
	}
	err = workTree.Checkout(&git.CheckoutOptions{
		Hash: *commit,
	})
	if err != nil {
		return nil, nil, err
	}

	config, _, err := caddycmd.LoadConfig(path.Join(adapterConfig.ClonePath, adapterConfig.Caddyfile), "caddyfile")
	if err != nil {
		return nil, nil, err
	}

	return config, nil, err
}

var _ caddyconfig.Adapter = (*Adapter)(nil)
