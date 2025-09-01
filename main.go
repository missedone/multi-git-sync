package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/lmittmann/tint"
	"gopkg.in/yaml.v3"
)

var (
	version string
	build   string
)

type Auth struct {
	User                 string `yaml:"user"`
	AccessToken          string `yaml:"accessToken"`
	PrivateKeyFile       string `yaml:"privateKeyFile"`
	PrivateKeyPassphrase string `yaml:"privateKeyPassphrase"`
}

type Repo struct {
	URL      string `yaml:"url"`
	Branch   string `yaml:"branch"`
	Depth    int    `yaml:"depth"`
	SubPath  string `yaml:"subPath"`
	Auth     Auth   `yaml:"auth"`
	DestDir  string `yaml:"destDir"`
	Schedule string `yaml:"schedule"`
}

func (r Repo) String() string {
	return fmt.Sprintf("URL:%s, Branch:%s, SubPath:%s, DestDir:%s", r.URL, r.Branch, r.SubPath, r.DestDir)
}

type Config struct {
	Repos []Repo `yaml:"repos"`
}

// Checkout a Branch
func main() {
	// set the colorful logger as the global logger with custom options
	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.DateTime,
		}),
	))

	configFile := flag.String("config", "", "the config file path")
	flag.Usage = func() {
		slog.Info(fmt.Sprintf("Version: %s-%s\nUsage: %s [-config=CONFIG_FILE]\n", version, build, os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()
	slog.Info(fmt.Sprintf("%s version %s-%s", os.Args[0], version, build))

	if configFile == nil || *configFile == "" {
		slog.Error("config file is required.")
		flag.PrintDefaults()
		os.Exit(1)
	}
	config, err := os.ReadFile(*configFile)
	if err != nil {
		slog.Error("failed to read config file", "file", *configFile, slog.Any("error", err))
		os.Exit(1)
	}
	c, err := parseConfig(config)
	if err != nil {
		slog.Error("failed to parse config", "file", *configFile, slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("start scheduler")
	if err := execute(c); err != nil {
		slog.Error("failed to execute the scheduler", slog.Any("error", err))
		os.Exit(1)
	}
}

func parseConfig(config []byte) (*Config, error) {
	tmpl, err := template.New("configTemplate").Funcs(template.FuncMap{
		"getEnv": os.Getenv,
	}).Parse(string(config))
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, nil); err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(buf.Bytes(), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func execute(conf *Config) error {
	s, err := gocron.NewScheduler()
	if err != nil {
		return err
	}
	defer func() { _ = s.Shutdown() }()

	for _, repo := range conf.Repos {
		_, err := s.NewJob(
			gocron.CronJob(
				// standard cron tab parsing
				repo.Schedule,
				false,
			),
			gocron.NewTask(
				func() {
					err := sync(repo)
					if err != nil {
						slog.Error("Sync git repo failed.",
							slog.Any("Repo", repo),
							slog.Any("error", err),
						)
					} else {
						ref, err := head(repo.DestDir)
						if err == nil {
							slog.Info(fmt.Sprintf("git show-ref --head HEAD: %s", ref.Hash()), slog.Any("Repo", repo))
						}
						slog.Info("Sync git repo completed.", slog.Any("Repo", repo))
					}
				},
			),
		)
		if err != nil {
			return err
		}
	}

	s.Start()

	blockUntilSignal()
	slog.Info("job interrupted, shutdown scheduler now.")
	return s.Shutdown()
}

func sync(repo Repo) error {
	var err error
	var gitAuth transport.AuthMethod
	if strings.HasPrefix(repo.URL, "http") {
		gitAuth = &http.BasicAuth{
			Username: repo.Auth.User,
			Password: repo.Auth.AccessToken,
		}
	} else {
		keyFile := repo.Auth.PrivateKeyFile
		if strings.HasPrefix(keyFile, "~/") {
			homedir, _ := os.UserHomeDir()
			keyFile = filepath.Join(homedir, keyFile[2:])
		}
		gitAuth, err = ssh.NewPublicKeysFromFile(repo.Auth.User, keyFile, repo.Auth.PrivateKeyPassphrase)
		if err != nil {
			return err
		}
	}
	r, err := git.PlainOpen(repo.DestDir)
	if err != nil {
		slog.Info(fmt.Sprintf("git clone --no-checkout %s -b %s %s", repo.URL, repo.Branch, repo.DestDir),
			slog.Any("SubPath", repo.SubPath),
		)
		err = checkout(repo.URL, repo.Branch, repo.SubPath, gitAuth, repo.DestDir, repo.Depth)
	} else {
		if repo.Depth <= 0 {
			slog.Info(fmt.Sprintf("git pull %s", repo.DestDir), slog.Any("Repo", repo))
			err = pull(r, gitAuth)
		} else {
			slog.Info(fmt.Sprintf("git fetch --depth %d", repo.Depth), slog.Any("Repo", repo))
			err = fetch(r, repo.SubPath, repo.Branch, gitAuth, repo.Depth)
		}
	}
	return err
}

func checkout(url, branch, subPath string, auth transport.AuthMethod, destDir string, depth int) error {
	r, err := git.PlainClone(destDir, false, &git.CloneOptions{
		Auth:       auth,
		URL:        url,
		Depth:      depth,
		NoCheckout: true,
		Progress:   os.Stdout,
	})
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	branchRefName := plumbing.NewBranchReferenceName(branch)
	branchCoOpts := git.CheckoutOptions{
		Branch: branchRefName,
		Force:  true,
	}
	if subPath != "" {
		branchCoOpts.SparseCheckoutDirectories = []string{subPath}
	}
	err = w.Checkout(&branchCoOpts)

	return err
}

func pull(r *git.Repository, auth transport.AuthMethod) error {
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	err = w.Pull(&git.PullOptions{
		RemoteName:   git.DefaultRemoteName,
		Auth:         auth,
		SingleBranch: true,
		Force:        true,
		Progress:     os.Stdout,
	})
	if err == nil || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	} else {
		return err
	}
}

func fetch(r *git.Repository, subPath, branch string, auth transport.AuthMethod, depth int) error {
	if err := r.Fetch(&git.FetchOptions{
		RemoteName: git.DefaultRemoteName,
		Auth:       auth,
		Depth:      depth,
		Progress:   os.Stdout,
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	remoteRef, err := r.Reference(plumbing.NewRemoteReferenceName(git.DefaultRemoteName, branch), true)
	if err != nil {
		return err
	}
	err = w.ResetSparsely(&git.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   git.HardReset,
	}, []string{subPath})

	return err
}

func head(path string) (*plumbing.Reference, error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return r.Head()
}

func blockUntilSignal() {
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done
}
