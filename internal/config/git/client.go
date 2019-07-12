package git

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

const (
	defaultUsername     = "samsahai"
	defaultCloneTimeout = 30 * time.Second
	defaultPullTimeout  = 15 * time.Second
	//defaultPushTimeout  = 15 * time.Second

	baseDir = "gitstorage"

	//authorName  = "samsahai"
	//authorEmail = "samsahai@samsahai.io"
)

// Client manages client side of git
type Client struct {
	url     string
	dirPath string
	option  gitOptions
	ep      *transport.Endpoint
}

type gitOptions struct {
	auth         *http.BasicAuth
	refName      string
	cloneDepth   int
	cloneTimeout *time.Duration
	pullTimeout  *time.Duration
	pushTimeout  *time.Duration
}

type Option func(client *Client)

func WithAuth(username, passwordOrToken string) Option {
	if username == "" {
		username = defaultUsername
	}

	return func(c *Client) {
		c.option.auth = &http.BasicAuth{
			Username: username, // this can be anything except an empty string
			Password: passwordOrToken,
		}
	}
}

func WithReferenceName(refName string) Option {
	return func(c *Client) {
		c.option.refName = refName
	}
}

func WithCloneDepth(depth int) Option {
	return func(c *Client) {
		c.option.cloneDepth = depth
	}
}

func WithCloneTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.option.cloneTimeout = &timeout
	}
}

func WithPullTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.option.pullTimeout = &timeout
	}
}

func WithPushTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.option.pushTimeout = &timeout
	}
}

// NewClient creates a new git client
func NewClient(dirName, url string, opts ...Option) (*Client, error) {
	pwd, _ := os.Getwd()
	dirPath := path.Join(pwd, baseDir, dirName)

	client := &Client{
		url:     url,
		dirPath: dirPath,
	}

	for _, opt := range opts {
		opt(client)
	}

	var err error
	client.ep, err = transport.NewEndpoint(url)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create endpoint from git url `%s`", url)
	}

	return client, nil
}

func (c *Client) GetPath() string {
	return strings.Replace(strings.TrimLeft(c.ep.Path, "/"), ".git", "", 1)
}

func (c *Client) GetName() string {
	s := strings.Split(c.GetPath(), "/")
	return s[len(s)-1]
}

func (c *Client) GetBranchName() string {
	if strings.Contains(c.option.refName, "refs/heads/") {
		return strings.Split(c.option.refName, "refs/heads/")[1]
	}
	return c.option.refName
}

// GetGitParams returns params of git client
func (c *Client) GetGitParams() (url, ref string, cloneDepth int) {
	return c.url, c.option.refName, c.option.cloneDepth
}

// GetDirectoryPath returns a directory path which project was cloned
func (c *Client) GetDirectoryPath() string {
	return c.dirPath
}

// GetHeadRevision returns a revision of HEAD commit
func (c *Client) GetHeadRevision() (string, error) {
	r, err := gogit.PlainOpen(c.dirPath)
	if err != nil {
		return "", err
	}

	h, err := r.ResolveRevision(plumbing.Revision("HEAD"))
	if err != nil {
		return "", err
	}

	return h.String(), nil
}

// Clone clones a repository into defined path
func (c *Client) Clone() error {
	// delete exist directory and create a new directory
	if err := createNewDirectory(c.dirPath); err != nil {
		return err
	}

	errCh := make(chan error)
	go func() {
		// clone the repository into defined path
		_, err := gogit.PlainClone(c.dirPath, false, &gogit.CloneOptions{
			URL:           c.url,
			Auth:          c.option.auth,
			ReferenceName: plumbing.ReferenceName(c.option.refName),
			Depth:         c.option.cloneDepth,
		})
		errCh <- err
	}()

	timeout := defaultCloneTimeout
	if c.option.cloneTimeout != nil {
		timeout = *c.option.cloneTimeout
	}

	select {
	case err := <-errCh:
		return err
	case <-time.After(timeout):
		return errors.Wrap(s2herrors.ErrGitCloneTimeout, fmt.Sprintf("git cloning %s timeout", c.url))
	}
}

// Pull pulls changes from git repository
func (c *Client) Pull() error {
	// open an already existing repository
	r, err := gogit.PlainOpen(c.dirPath)
	if err != nil {
		return err
	}

	// get the working directory for the repository
	w, err := r.Worktree()
	if err != nil {
		return err
	}

	errCh := make(chan error)
	go func() {
		// pull the latest changes from the origin remote and merge into the current branch
		errCh <- w.Pull(&gogit.PullOptions{
			RemoteName:    "origin",
			Auth:          c.option.auth,
			ReferenceName: plumbing.ReferenceName(c.option.refName),
		})
	}()

	timeout := defaultPullTimeout
	if c.option.pushTimeout != nil {
		timeout = *c.option.pullTimeout
	}

	select {
	case err := <-errCh:
		return err
	case <-time.After(timeout):
		return errors.Wrap(s2herrors.ErrGitPullTimeout, fmt.Sprintf("git pulling %s timeout", c.url))
	}
}

// Clean deletes the git directory
func (c *Client) Clean() error {
	if err := deleteDirectory(c.dirPath); err != nil {
		return err
	}

	return nil
}

// CommitAndPush commits and pushes changes to the repository
//func (c *Client) CommitAndPush(content []byte, path, commitName string) error {
//	errCh := make(chan error)
//	go func() {
//		if err := c.commit(content, path, commitName); err != nil {
//			errCh <- err
//		}
//
//		// open an already existing repository
//		r, err := gogit.PlainOpen(c.dirPath)
//		if err != nil {
//			errCh <- err
//		}
//
//		// push the changes to repository
//		if err = r.Push(&gogit.PushOptions{Auth: c.option.auth}); err != nil {
//			errCh <- err
//		}
//
//		errCh <- nil
//	}()
//
//	timeout := defaultPushTimeout
//	if c.option.pushTimeout != nil {
//		timeout = *c.option.pushTimeout
//	}
//
//	select {
//	case err := <-errCh:
//		return err
//	case <-time.After(timeout):
//		return errors.Wrap(s2herrors.ErrGitPushTimeout, fmt.Sprintf("git pushing %s timeout", c.url))
//	}
//}

//func (c *Client) commit(content []byte, commitFilePath, commitName string) error {
//	// open an already existing repository
//	r, err := gogit.PlainOpen(c.dirPath)
//	if err != nil {
//		return err
//	}
//
//	// get the working directory for the repository
//	w, err := r.Worktree()
//	if err != nil {
//		return err
//	}
//
//	// write data into a file
//	filename := filepath.Join(c.dirPath, commitFilePath)
//	err = ioutil.WriteFile(filename, content, 0644)
//	if err != nil {
//		return err
//	}
//
//	// add file
//	if _, err = w.Add(commitFilePath); err != nil {
//		return err
//	}
//
//	// commit the current staging area to the repository
//	_, err = w.Commit(commitName, &gogit.CommitOptions{
//		Author: &object.Signature{
//			Name:  authorName,
//			Email: authorEmail,
//			When:  time.Now(),
//		},
//	})
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

func createNewDirectory(dirPath string) error {
	if err := deleteDirectory(dirPath); err != nil {
		return err
	}

	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return err
	}

	return nil
}

func deleteDirectory(dirPath string) error {
	if err := os.RemoveAll(dirPath); err != nil {
		return err
	}

	return nil
}
