module github.com/form3tech-oss/github-team-approver

go 1.13

replace github.com/gregjones/httpcache => github.com/form3tech-oss/httpcache v0.0.0-20190708110905-85712625ba05
replace github.com/form3tech-oss/github-team-approver-commons => /Users/janakerman/go/src/github.com/form3tech-oss/github-team-approver-commons

require (
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/form3tech-oss/github-team-approver-commons v1.3.0
	github.com/form3tech-oss/go-pact-testing v1.4.1
	github.com/form3tech-oss/logrus-logzio-hook v1.0.0
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/go-github/v28 v28.1.1
	github.com/google/tcpproxy v0.0.0-20180808230851-dfa16c61dad2
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/gregjones/httpcache v0.0.0-00010101000000-000000000000
	github.com/hashicorp/go-version v1.2.1-0.20190424083514-192140e6f3e6 // indirect
	github.com/logzio/logzio-go v0.0.0-20190916115104-4cd568a9b6d6
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/sirupsen/logrus v1.8.1
	github.com/slack-go/slack v0.6.5
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550 // indirect
	golang.org/x/net v0.0.0-20200226121028-0de0cce0169b // indirect
)
