module github.com/form3tech-oss/github-team-approver

go 1.13

replace github.com/gregjones/httpcache => github.com/form3tech-oss/httpcache v0.0.0-20190708110905-85712625ba05

require (
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/form3tech-oss/github-team-approver-commons v1.3.2
	github.com/form3tech-oss/go-pact-testing v1.4.1
	github.com/google/go-github/v42 v42.0.0
	github.com/google/tcpproxy v0.0.0-20180808230851-dfa16c61dad2
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/gregjones/httpcache v0.0.0-00010101000000-000000000000
	github.com/hashicorp/go-version v1.2.1-0.20190424083514-192140e6f3e6 // indirect
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/sirupsen/logrus v1.8.1
	github.com/slack-go/slack v0.10.3
	github.com/spf13/viper v1.11.0
	github.com/stretchr/testify v1.7.1
)
