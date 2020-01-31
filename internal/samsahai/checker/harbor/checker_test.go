package harbor

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hhttp "github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/unittest"
)

func TestHarborChecker(t *testing.T) {
	unittest.InitGinkgo(t, "Harbor Checker")
}

var _ = Describe("Harbor Checker", func() {
	g := NewWithT(GinkgoT())

	var checker internal.DesiredComponentChecker
	//var err error
	//var accessToken = "123456"
	var server *httptest.Server

	BeforeEach(func() {
		checker = New(s2hhttp.WithSkipTLSVerify())
	})

	AfterEach(func() {

	})

	It("Should returns 'harbor' as name", func() {
		Expect(checker.GetName()).To(Equal("harbor"))
	})

	It("Should successfully get new version from harbor", func(done Done) {
		defer close(done)
		server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_, err := ioutil.ReadAll(r.Body)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = w.Write([]byte(`
[
  {
    "digest": "sha256:0d6b5b6238c0f8a83faa120462249bd167009e3dca7b82be7d852017089b335d",
    "name": "1.13.5-3.0.0-beta.1",
    "size": 235034692,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "19.03.1",
    "author": "it-devops-container@agoda.com",
    "created": "2019-08-28T01:08:07.834151787Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com",
        "org.label-schema.build-date": "20190801",
        "org.label-schema.license": "GPLv2",
        "org.label-schema.name": "CentOS Base Image",
        "org.label-schema.schema-version": "1.0",
        "org.label-schema.vendor": "CentOS"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:0d6b5b6238c0f8a83faa120462249bd167009e3dca7b82be7d852017089b335d",
      "scan_status": "finished",
      "job_id": 1690447,
      "severity": 4,
      "components": {
        "total": 198,
        "summary": [
          {
            "severity": 3,
            "count": 7
          },
          {
            "severity": 1,
            "count": 173
          },
          {
            "severity": 4,
            "count": 18
          }
        ]
      },
      "details_key": "0ce4c55274c09c6fc2b9bbbc1cd48505e4b5bd2886cb87a5859a05704e471cf4",
      "creation_time": "2019-08-28T01:08:21.524831Z",
      "update_time": "2019-08-28T08:08:32.097541Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:b7e2dfe9d9b6c2574fe421aeee90af32ab74a1ddf42217c69e2e6c88198bf034",
    "name": "1.13.5-3.0.0-alpha.1",
    "size": 270823831,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "18.09.6",
    "author": "it-devops-container@agoda.com",
    "created": "2019-05-30T06:58:54.369635773Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com",
        "org.label-schema.build-date": "20190305",
        "org.label-schema.license": "GPLv2",
        "org.label-schema.name": "CentOS Base Image",
        "org.label-schema.schema-version": "1.0",
        "org.label-schema.vendor": "CentOS"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:b7e2dfe9d9b6c2574fe421aeee90af32ab74a1ddf42217c69e2e6c88198bf034",
      "scan_status": "finished",
      "job_id": 1596731,
      "severity": 5,
      "components": {
        "total": 198,
        "summary": [
          {
            "severity": 5,
            "count": 4
          },
          {
            "severity": 1,
            "count": 172
          },
          {
            "severity": 4,
            "count": 15
          },
          {
            "severity": 3,
            "count": 7
          }
        ]
      },
      "details_key": "e06f2ba8580c564938aef3aabfd55afe0237710a9093323b0263f20a8f54ea04",
      "creation_time": "2019-05-30T06:59:13.300556Z",
      "update_time": "2019-08-14T15:56:17.902114Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:4ac349673b953c03ccca57d134c6bfa3e261d52a5b68135d066035745e91828d",
    "name": "1.13.5-2.13.1",
    "size": 312412277,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "18.09.7",
    "author": "it-devops-container@agoda.com",
    "created": "2019-08-16T00:50:57.865244942Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com",
        "org.label-schema.build-date": "20190305",
        "org.label-schema.license": "GPLv2",
        "org.label-schema.name": "CentOS Base Image",
        "org.label-schema.schema-version": "1.0",
        "org.label-schema.vendor": "CentOS"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:4ac349673b953c03ccca57d134c6bfa3e261d52a5b68135d066035745e91828d",
      "scan_status": "finished",
      "job_id": 1676031,
      "severity": 4,
      "components": {
        "total": 198,
        "summary": [
          {
            "severity": 3,
            "count": 7
          },
          {
            "severity": 4,
            "count": 18
          },
          {
            "severity": 1,
            "count": 173
          }
        ]
      },
      "details_key": "aa2d064b07e413f70bc600467638f2d70099e87b6be599c7cd93cc6275073ce3",
      "creation_time": "2019-08-16T00:51:27.172774Z",
      "update_time": "2019-08-16T07:51:53.443639Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:4ac349673b953c03ccca57d134c6bfa3e261d52a5b68135d066035745e91828d",
    "name": "hkci-k8s-a",
    "size": 312412277,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "18.09.7",
    "author": "it-devops-container@agoda.com",
    "created": "2019-08-16T00:50:57.865244942Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com",
        "org.label-schema.build-date": "20190305",
        "org.label-schema.license": "GPLv2",
        "org.label-schema.name": "CentOS Base Image",
        "org.label-schema.schema-version": "1.0",
        "org.label-schema.vendor": "CentOS"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:4ac349673b953c03ccca57d134c6bfa3e261d52a5b68135d066035745e91828d",
      "scan_status": "finished",
      "job_id": 1676031,
      "severity": 4,
      "components": {
        "total": 198,
        "summary": [
          {
            "severity": 3,
            "count": 7
          },
          {
            "severity": 4,
            "count": 18
          },
          {
            "severity": 1,
            "count": 173
          }
        ]
      },
      "details_key": "aa2d064b07e413f70bc600467638f2d70099e87b6be599c7cd93cc6275073ce3",
      "creation_time": "2019-08-16T00:51:27.172774Z",
      "update_time": "2019-08-16T07:51:53.443639Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:0b6fb8f43bb4f63b7d61c7cddbba6dd9455c8c2d0234306f9f204a421c31a3e3",
    "name": "1.12.3-2.12.3",
    "size": 144031059,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "18.09.2",
    "author": "it-devops-container@agoda.com",
    "created": "2019-03-14T08:43:49.669466959Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:0b6fb8f43bb4f63b7d61c7cddbba6dd9455c8c2d0234306f9f204a421c31a3e3",
      "scan_status": "finished",
      "job_id": 1627978,
      "severity": 2,
      "components": {
        "total": 59,
        "summary": [
          {
            "severity": 1,
            "count": 55
          },
          {
            "severity": 2,
            "count": 4
          }
        ]
      },
      "details_key": "b6d1ed91aeb9af46f4bc1cdbc167973de4a5d40b638d9a648ce58e1502fe223f",
      "creation_time": "2019-03-14T08:44:02.225502Z",
      "update_time": "2019-09-08T08:37:37.591375Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:7da412835f70d13714bf0d53b757bc5890bb2961811c56792f70b1371cb9a820",
    "name": "helm3",
    "size": 235036937,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "19.03.1",
    "author": "it-devops-container@agoda.com",
    "created": "2019-08-29T07:46:16.388165365Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com",
        "org.label-schema.build-date": "20190801",
        "org.label-schema.license": "GPLv2",
        "org.label-schema.name": "CentOS Base Image",
        "org.label-schema.schema-version": "1.0",
        "org.label-schema.vendor": "CentOS"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:7da412835f70d13714bf0d53b757bc5890bb2961811c56792f70b1371cb9a820",
      "scan_status": "finished",
      "job_id": 1693280,
      "severity": 4,
      "components": {
        "total": 198,
        "summary": [
          {
            "severity": 3,
            "count": 7
          },
          {
            "severity": 1,
            "count": 173
          },
          {
            "severity": 4,
            "count": 18
          }
        ]
      },
      "details_key": "2e6ca0f28bb87a0c282f2e3cb098381262e448dec3428f552d5286a0469a022c",
      "creation_time": "2019-08-29T07:46:31.33717Z",
      "update_time": "2019-08-29T14:46:40.367044Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:7da412835f70d13714bf0d53b757bc5890bb2961811c56792f70b1371cb9a820",
    "name": "1.13.5-3.0.0-beta.2",
    "size": 235036937,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "19.03.1",
    "author": "it-devops-container@agoda.com",
    "created": "2019-08-29T07:46:16.388165365Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com",
        "org.label-schema.build-date": "20190801",
        "org.label-schema.license": "GPLv2",
        "org.label-schema.name": "CentOS Base Image",
        "org.label-schema.schema-version": "1.0",
        "org.label-schema.vendor": "CentOS"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:7da412835f70d13714bf0d53b757bc5890bb2961811c56792f70b1371cb9a820",
      "scan_status": "finished",
      "job_id": 1693280,
      "severity": 4,
      "components": {
        "total": 198,
        "summary": [
          {
            "severity": 3,
            "count": 7
          },
          {
            "severity": 1,
            "count": 173
          },
          {
            "severity": 4,
            "count": 18
          }
        ]
      },
      "details_key": "2e6ca0f28bb87a0c282f2e3cb098381262e448dec3428f552d5286a0469a022c",
      "creation_time": "2019-08-29T07:46:31.33717Z",
      "update_time": "2019-08-29T14:46:40.367044Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:6cd2742d29ddd72d1ef3a52fbf29410d0313ec975406c7510ac28a4f1579a3b1",
    "name": "1.13.4-2.13.0",
    "size": 128040845,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "18.09.3",
    "author": "it-devops-container@agoda.com",
    "created": "2019-03-15T03:11:04.509964658Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:6cd2742d29ddd72d1ef3a52fbf29410d0313ec975406c7510ac28a4f1579a3b1",
      "scan_status": "finished",
      "job_id": 1627979,
      "severity": 2,
      "components": {
        "total": 59,
        "summary": [
          {
            "severity": 1,
            "count": 55
          },
          {
            "severity": 2,
            "count": 4
          }
        ]
      },
      "details_key": "2e9a6350679d697597b94e602e9a2ceefc8f366cf2d3b8bf1808332a19684269",
      "creation_time": "2019-03-15T03:11:15.275906Z",
      "update_time": "2019-09-08T08:37:37.599993Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:83b790d58d12ec623a94e2d05f320edee39eec8ef1d9d84f9466db7110d48c87",
    "name": "1.13.5-3.0.0-alpha.2",
    "size": 292101534,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "19.03.1",
    "author": "it-devops-container@agoda.com",
    "created": "2019-08-15T02:06:49.062773925Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com",
        "org.label-schema.build-date": "20190305",
        "org.label-schema.license": "GPLv2",
        "org.label-schema.name": "CentOS Base Image",
        "org.label-schema.schema-version": "1.0",
        "org.label-schema.vendor": "CentOS"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:83b790d58d12ec623a94e2d05f320edee39eec8ef1d9d84f9466db7110d48c87",
      "scan_status": "finished",
      "job_id": 1674785,
      "severity": 4,
      "components": {
        "total": 198,
        "summary": [
          {
            "severity": 3,
            "count": 7
          },
          {
            "severity": 1,
            "count": 173
          },
          {
            "severity": 4,
            "count": 18
          }
        ]
      },
      "details_key": "f3a388f7a308e3628ba9a886c2de3ac5f52a76a8c1f6683770e550ef460f6fb4",
      "creation_time": "2019-08-15T02:07:11.252318Z",
      "update_time": "2019-08-15T09:07:22.495691Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:8ed46edf5718568a1ca90b6e92b0c70ea33fabf29bc6b66f629e53097619bae4",
    "name": "1.14.3-2.14.1",
    "size": 312243192,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "18.09.6",
    "author": "it-devops-container@agoda.com",
    "created": "2019-06-21T07:30:15.942729727Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com",
        "org.label-schema.build-date": "20190305",
        "org.label-schema.license": "GPLv2",
        "org.label-schema.name": "CentOS Base Image",
        "org.label-schema.schema-version": "1.0",
        "org.label-schema.vendor": "CentOS"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:8ed46edf5718568a1ca90b6e92b0c70ea33fabf29bc6b66f629e53097619bae4",
      "scan_status": "finished",
      "job_id": 1596730,
      "severity": 5,
      "components": {
        "total": 198,
        "summary": [
          {
            "severity": 4,
            "count": 16
          },
          {
            "severity": 3,
            "count": 7
          },
          {
            "severity": 1,
            "count": 172
          },
          {
            "severity": 5,
            "count": 3
          }
        ]
      },
      "details_key": "618aea4b25faf537f1fd9787834e60c6d3d200b6dc5c38e792c1ae3270d0cb46",
      "creation_time": "2019-06-21T07:30:37.628299Z",
      "update_time": "2019-08-16T04:11:26.532871Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:f5cbb44b93ee04832e17bfdb0596e200402b7b4fdd911085a3fa359eb23baa93",
    "name": "1.13.5-2.13.0",
    "size": 130171734,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "18.09.3",
    "author": "it-devops-container@agoda.com",
    "created": "2019-04-03T01:14:01.728706915Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:f5cbb44b93ee04832e17bfdb0596e200402b7b4fdd911085a3fa359eb23baa93",
      "scan_status": "finished",
      "job_id": 1596726,
      "severity": 2,
      "components": {
        "total": 60,
        "summary": [
          {
            "severity": 2,
            "count": 4
          },
          {
            "severity": 1,
            "count": 56
          }
        ]
      },
      "details_key": "3e4f92e8e6be29b417636061f7c3997cb65e0508bfe7c6c25fb50da9ab9096ba",
      "creation_time": "2019-04-03T01:14:11.365679Z",
      "update_time": "2019-09-09T20:57:36.252182Z"
    },
    "labels": []
  }
]
`))
			g.Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		repo := strings.Replace(server.URL, "https://", "", 1) + "/aiab/kubectl"
		version, err := checker.GetVersion(repo, "kubectl", "1\\.13\\..+")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(version).To(Equal("1.13.5-3.0.0-beta.2"))
	})

	It("Should correctly ensure version", func(done Done) {
		defer close(done)
		server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_, err := ioutil.ReadAll(r.Body)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = w.Write([]byte(`
[
  {
    "digest": "sha256:0d6b5b6238c0f8a83faa120462249bd167009e3dca7b82be7d852017089b335d",
    "name": "1.13.5-3.0.0-beta.1",
    "size": 235034692,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "19.03.1",
    "author": "it-devops-container@agoda.com",
    "created": "2019-08-28T01:08:07.834151787Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com",
        "org.label-schema.build-date": "20190801",
        "org.label-schema.license": "GPLv2",
        "org.label-schema.name": "CentOS Base Image",
        "org.label-schema.schema-version": "1.0",
        "org.label-schema.vendor": "CentOS"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:0d6b5b6238c0f8a83faa120462249bd167009e3dca7b82be7d852017089b335d",
      "scan_status": "finished",
      "job_id": 1690447,
      "severity": 4,
      "components": {
        "total": 198,
        "summary": [
          {
            "severity": 3,
            "count": 7
          },
          {
            "severity": 1,
            "count": 173
          },
          {
            "severity": 4,
            "count": 18
          }
        ]
      },
      "details_key": "0ce4c55274c09c6fc2b9bbbc1cd48505e4b5bd2886cb87a5859a05704e471cf4",
      "creation_time": "2019-08-28T01:08:21.524831Z",
      "update_time": "2019-08-28T08:08:32.097541Z"
    },
    "labels": []
  },
  {
    "digest": "sha256:b7e2dfe9d9b6c2574fe421aeee90af32ab74a1ddf42217c69e2e6c88198bf034",
    "name": "1.13.5-3.0.0-alpha.1",
    "size": 270823831,
    "architecture": "amd64",
    "os": "linux",
    "docker_version": "18.09.6",
    "author": "it-devops-container@agoda.com",
    "created": "2019-05-30T06:58:54.369635773Z",
    "config": {
      "labels": {
        "maintainer": "it-devops-container@agoda.com",
        "org.label-schema.build-date": "20190305",
        "org.label-schema.license": "GPLv2",
        "org.label-schema.name": "CentOS Base Image",
        "org.label-schema.schema-version": "1.0",
        "org.label-schema.vendor": "CentOS"
      }
    },
    "signature": null,
    "scan_overview": {
      "image_digest": "sha256:b7e2dfe9d9b6c2574fe421aeee90af32ab74a1ddf42217c69e2e6c88198bf034",
      "scan_status": "finished",
      "job_id": 1596731,
      "severity": 5,
      "components": {
        "total": 198,
        "summary": [
          {
            "severity": 5,
            "count": 4
          },
          {
            "severity": 1,
            "count": 172
          },
          {
            "severity": 4,
            "count": 15
          },
          {
            "severity": 3,
            "count": 7
          }
        ]
      },
      "details_key": "e06f2ba8580c564938aef3aabfd55afe0237710a9093323b0263f20a8f54ea04",
      "creation_time": "2019-05-30T06:59:13.300556Z",
      "update_time": "2019-08-14T15:56:17.902114Z"
    },
    "labels": []
  }
]
`))
			g.Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		repo := strings.Replace(server.URL, "https://", "", 1) + "/aiab/kubectl"
		err := checker.EnsureVersion(repo, "kubectl", "1.13.5-3.0.0-beta.1")
		g.Expect(err).NotTo(HaveOccurred())

		err = checker.EnsureVersion(repo, "kubectl", "1.13.5-3.0.0-beta.1-missing")
		g.Expect(s2herrors.IsImageNotFound(err)).To(BeTrue())
	})

	Specify("Invalid json response", func(done Done) {
		defer close(done)
		server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_, err := ioutil.ReadAll(r.Body)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = w.Write([]byte(``))
			g.Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		repo := strings.Replace(server.URL, "https://", "", 1) + "/aiab/kubectl"
		_, err := checker.GetVersion(repo, "kubectl", "1\\.13\\..+")
		g.Expect(err).NotTo(BeNil())
	})
})
