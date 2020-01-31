package config

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/config"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

var _ = Describe("config manager test [e2e]", func() {
	var configMgr internal.ConfigManager

	gitUsername, gitPassword := os.Getenv("TEST_GIT_USERNAME"), os.Getenv("TEST_GIT_PASSWORD")

	gitStorage := s2hv1beta1.GitStorage{
		URL:          "https://github.com/agoda-com/samsahai-example.git",
		Path:         "configs",
		CloneDepth:   1,
		CloneTimeout: &metav1.Duration{Duration: 15 * time.Second},
		PullTimeout:  &metav1.Duration{Duration: 5 * time.Second},
		PushTimeout:  &metav1.Duration{Duration: 5 * time.Second},
	}

	gitCred := &s2hv1beta1.UsernamePasswordCredential{
		Username: gitUsername,
		Password: gitPassword,
	}

	AfterEach(func() {
		By("Cleaning up git repo")
		Expect(configMgr.Clean()).NotTo(HaveOccurred())
	}, 20)

	It("should create config manager with git client and sync successfully", func() {
		By("Checking config manager has been created")
		err := wait.PollImmediate(500*time.Millisecond, 15*time.Second, func() (ok bool, err error) {
			configMgr, err = config.NewWithGit("samsahai-example", gitStorage, gitCred)
			if err != nil {
				if s2herrors.IsErrGitCloning(err) {
					return false, nil
				}
				return false, err
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(configMgr).NotTo(BeNil())
		Expect(len(configMgr.GetComponents())).ShouldNot(BeZero())

		By("Checking config manager has been synced")
		err = wait.PollImmediate(500*time.Millisecond, 5*time.Second, func() (ok bool, err error) {
			err = configMgr.Sync()
			if err != nil && s2herrors.IsErrGitPulling(err) {
				return false, nil
			}

			return true, nil
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(configMgr.GetComponents())).ShouldNot(BeZero())

		By("Checking git latest revision")
		rev := configMgr.GetGitLatestRevision()
		Expect(len(rev)).Should(Equal(40))
	}, 30)
})
