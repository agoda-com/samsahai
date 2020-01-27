module github.com/agoda-com/samsahai

go 1.13

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/docker/distribution v2.7.1+incompatible
	github.com/fluxcd/flux v1.17.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-chi/chi v4.0.2+incompatible // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/golang/groupcache v0.0.0-20191227052852-215e87163ea7 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.4.0
	github.com/google/uuid v1.1.1
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/julienschmidt/httprouter v1.2.0
	github.com/jvsteiner/multilock v0.0.0-20190621192552-684f2fa3399f
	github.com/lusis/go-slackbot v0.0.0-20180109053408-401027ccfef5 // indirect
	github.com/lusis/slack-test v0.0.0-20180109053238-3c758769bfa6 // indirect
	github.com/nlopes/slack v0.5.0
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.3.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.3.2
	github.com/swaggo/http-swagger v0.0.0-20190614090009-c2865af9083e
	github.com/swaggo/swag v1.6.3
	github.com/tidwall/gjson v1.2.1
	github.com/tidwall/match v1.0.1 // indirect
	github.com/twitchtv/twirp v5.10.0+incompatible
	go.uber.org/atomic v1.4.0 // indirect
	golang.org/x/crypto v0.0.0-20200108215511-5d647ca15757 // indirect
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/sys v0.0.0-20200107162124-548cf772de50 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.2.7 // indirect
	helm.sh/helm/v3 v3.0.2
	k8s.io/api v0.17.0
	k8s.io/apiextensions-apiserver v0.17.0 // indirect
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/helm v2.16.1+incompatible
	k8s.io/utils v0.0.0-20200108110541-e2fb8e668047 // indirect
	sigs.k8s.io/controller-runtime v0.4.0
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v2.7.1+incompatible

	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309

	k8s.io/api => k8s.io/api v0.16.4 // kubernetes-1.16.4

	k8s.io/apimachinery => k8s.io/apimachinery v0.16.4 // kubernetes-1.16.4

	k8s.io/cli-runtime => k8s.io/cli-runtime v0.16.4 // kubernetes-1.16.4

	k8s.io/client-go => k8s.io/client-go v0.16.4 // kubernetes-1.16.4

	sigs.k8s.io/controller-tools => github.com/phantomnat/controller-tools v0.2.4-1
)
