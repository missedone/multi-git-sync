package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	assert "github.com/stretchr/testify/require"
)

//go:embed examples/sparse-checkout/config.yaml
var testSparseCheckConfig []byte

//go:embed examples/clone/config.yaml
var testCloneConfig []byte

//go:embed examples/sshclone/config.yaml
var testSshCloneConfig []byte

//go:embed examples/shallow/config.yaml
var testShallowCloneConfig []byte

func TestParseConfig(t *testing.T) {
	testToken := os.Getenv("TEST_PAT")
	err := os.Setenv("PAT", testToken)
	assert.NoError(t, err)

	c, err := parseConfig(testSparseCheckConfig)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(c.Repos))
	repo := c.Repos[0]
	assert.Equal(t, "https://github.com/missedone/multi-git-sync-test.git", repo.URL)
	assert.Equal(t, "main", repo.Branch)
	assert.Equal(t, "foo", repo.SubPath)
	assert.Equal(t, testToken, repo.Auth.AccessToken)
}

func TestParseConfigWithSshClone(t *testing.T) {
	c, err := parseConfig(testSshCloneConfig)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(c.Repos))
	repo := c.Repos[0]
	assert.Equal(t, "https://github.com/missedone/multi-git-sync-test.git", repo.URL)
	assert.Equal(t, "main", repo.Branch)
	assert.Equal(t, "git", repo.Auth.User)
	assert.Equal(t, "~/.ssh/id_rsa", repo.Auth.PrivateKeyFile)
}

func TestSyncWithFullClone(t *testing.T) {
	err := os.Setenv("PAT", os.Getenv("TEST_PAT"))
	c, err := parseConfig(testCloneConfig)
	assert.NoError(t, err)

	repo := c.Repos[0]
	err = sync(repo)
	assert.NoError(t, err)

	_, err = os.Stat("./out/clone/multi-git-sync-test/foo/readme.md")
	assert.NoError(t, err)
	_, err = os.Stat("./out/clone/multi-git-sync-test/bar/readme.md")
	assert.NoError(t, err)
	_, err = os.Stat("./out/clone/multi-git-sync-test/README.md")
	assert.NoError(t, err)

	err = sync(repo)
	assert.NoError(t, err, "Should be no error with 1st re-sync the full cloned repo")

	err = sync(repo)
	assert.NoError(t, err, "Should be no error with 2nd re-sync the full cloned repo")

	//
	// test git pull on full-clone repo
	//

	// checkout and modify the content
	tmpDir, err := os.MkdirTemp("", "multi-git-sync-ut-full-clone")
	assert.NoError(t, err)
	defer os.Remove(tmpDir)
	testStr := time.Now().String()
	testFile := "bar/readme.md"
	err = updateTestData(repo, tmpDir, testFile, testStr)
	assert.NoError(t, err)

	// now sync repo after content update
	err = sync(repo)
	assert.NoError(t, err, "Should be no error with re-sync the full-clone repo after content update")
	_, err = os.Stat("./out/clone/multi-git-sync-test/README.md")
	assert.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(tmpDir, testFile))
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(b), testStr), "`%s` should contain the latest content", testFile)
}

func TestSyncWithShallowClone(t *testing.T) {
	err := os.Setenv("PAT", os.Getenv("TEST_PAT"))
	c, err := parseConfig(testShallowCloneConfig)
	assert.NoError(t, err)

	repo := c.Repos[0]
	err = sync(repo)
	assert.NoError(t, err)

	_, err = os.Stat("./out/shallow/multi-git-sync-test/bar/readme.md")
	assert.NoError(t, err)
	_, err = os.Stat("./out/shallow/multi-git-sync-test/README.md")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	err = sync(repo)
	assert.NoError(t, err, "Should be no error with 1st re-sync the shallow cloned repo")

	err = sync(repo)
	assert.NoError(t, err, "Should be no error with 2nd re-sync the shallow cloned repo")

	//
	// test git pull on shallow-clone repo
	//

	// checkout and modify the content
	tmpDir, err := os.MkdirTemp("", "multi-git-sync-ut-shallow-clone")
	assert.NoError(t, err)
	defer os.Remove(tmpDir)
	testStr := time.Now().String()
	testFile := "bar/readme.md"
	err = updateTestData(repo, tmpDir, testFile, testStr)
	assert.NoError(t, err)

	// now sync repo after content update
	err = sync(repo)
	_, err = os.Stat("./out/shallow/multi-git-sync-test/README.md")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	b, err := os.ReadFile(filepath.Join(tmpDir, testFile))
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(b), testStr), "`%s` should contain the latest content", testFile)
}

func TestSyncWithSparseCheckout(t *testing.T) {
	err := os.Setenv("PAT", os.Getenv("TEST_PAT"))
	c, err := parseConfig(testSparseCheckConfig)
	assert.NoError(t, err)

	repo := c.Repos[0]
	err = sync(repo)
	assert.NoError(t, err)

	_, err = os.Stat("./out/sparse-checkout/multi-git-sync-test/foo/readme.md")
	assert.NoError(t, err)
	_, err = os.Stat("./out/sparse-checkout/multi-git-sync-test/README.md")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	// re-sync once
	err = sync(repo)
	assert.NoError(t, err, "Should be no error with 1st re-sync the sparse-checkout repo")

	// re-sync twice
	err = sync(repo)
	assert.NoError(t, err, "Should be no error with 2nd re-sync the sparse-checkout repo")

	//
	// test git pull on sparse repo
	//

	// checkout and modify the content
	tmpDir, err := os.MkdirTemp("", "multi-git-sync-ut-sparse")
	assert.NoError(t, err)
	defer os.Remove(tmpDir)
	testStr := time.Now().String()
	testFile := "foo/readme.md"
	err = updateTestData(repo, tmpDir, testFile, testStr)
	assert.NoError(t, err)

	// now sync sparse repo after content update
	err = sync(repo)
	assert.NoError(t, err, "Should be no error with re-sync the sparse-checkout repo after content update")
	_, err = os.Stat("./out/sparse-checkout/multi-git-sync-test/README.md")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	b, err := os.ReadFile(filepath.Join(tmpDir, testFile))
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(b), testStr), "`%s` should contain the latest content", testFile)
}

func TestExecute(t *testing.T) {
	t.SkipNow()
	err := os.Setenv("PAT", os.Getenv("TEST_PAT"))
	c, err := parseConfig(testSparseCheckConfig)
	assert.NoError(t, err)

	err = execute(c)
	assert.NoError(t, err)
}

func updateTestData(repo Repo, tmpDir, f, testStr string) error {
	auth := &http.BasicAuth{
		Username: repo.Auth.User,
		Password: repo.Auth.AccessToken,
	}
	err := checkout(repo.URL, repo.Branch, "", auth, tmpDir, 0)
	if err != nil {
		return err
	}
	testFile := filepath.Join(tmpDir, f)
	fi, err := os.Stat(testFile)
	if err != nil {
		return err
	}
	err = os.WriteFile(testFile, []byte(testStr), fi.Mode())
	if err != nil {
		return err
	}
	r, err := git.PlainOpen(tmpDir)
	if err != nil {
		return err
	}
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	_, _ = w.Add(f)
	_, err = w.Commit(fmt.Sprintf("ut %s %s ", f, testStr), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "CI Bot",
			Email: "cibot@example.org",
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	err = r.Push(&git.PushOptions{
		Auth: auth,
	})
	return err
}
