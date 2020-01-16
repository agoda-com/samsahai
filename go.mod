module github.com/agoda-com/samsahai

go 1.12

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30 // indirect
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/go-systemd v0.0.0-20190620071333-e64a0ec8b42a // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/fluxcd/flux v1.17.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-chi/chi v4.0.2+incompatible // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/gogo/protobuf v1.3.0 // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.1
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/julienschmidt/httprouter v1.2.0
	github.com/jvsteiner/multilock v0.0.0-20190621192552-684f2fa3399f
	github.com/lusis/go-slackbot v0.0.0-20180109053408-401027ccfef5 // indirect
	github.com/lusis/slack-test v0.0.0-20180109053238-3c758769bfa6 // indirect
	github.com/nlopes/slack v0.5.0
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/opentracing-contrib/go-stdlib v0.0.0-20190519235532-cf7a6c988dc9 // indirect
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.1.0
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/spf13/cobra v0.0.4
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.3.2
	github.com/stretchr/testify v1.4.0 // indirect
	github.com/swaggo/http-swagger v0.0.0-20190614090009-c2865af9083e
	github.com/swaggo/swag v1.6.3
	github.com/tidwall/gjson v1.2.1
	github.com/tidwall/match v1.0.1 // indirect
	github.com/tidwall/pretty v1.0.0 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/twitchtv/twirp v5.8.0+incompatible
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/bbolt v1.3.3 // indirect
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20190911031432-227b76d455e7 // indirect
	golang.org/x/net v0.0.0-20190918130420-a8b05e9114ab
	golang.org/x/sys v0.0.0-20190919044723-0c1ff786ef13 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/src-d/go-git.v4 v4.13.1
	k8s.io/api v0.0.0-20190708174958-539a33f6e817
	k8s.io/apimachinery v0.0.0-20190719140911-bfcf53abc9f8
	k8s.io/apiserver v0.0.0-20190721103406-1e59c150c171 // indirect
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/helm v2.13.1+incompatible
	k8s.io/kube-openapi v0.0.0-20190918143330-0270cf2f1c1d // indirect
	k8s.io/kubernetes v1.13.4
	k8s.io/utils v0.0.0-20190712204705-3dccf664f023 // indirect
	sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/testing_frameworks v0.1.1 // indirect
)

replace github.com/docker/distribution => github.com/docker/distribution v2.7.1+incompatible

replace k8s.io/client-go => k8s.io/client-go v10.0.0+incompatible

replace k8s.io/code-generator => k8s.io/code-generator v0.0.0-20181117043124-c2090bec4d9b

replace k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628

replace k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190531163647-86e316accdba

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190606210424-3341abf3c63d

replace sigs.k8s.io/controller-tools => github.com/phantomnat/controller-tools v0.1.11-1
