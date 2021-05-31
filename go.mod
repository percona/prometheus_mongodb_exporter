module github.com/percona/mongodb_exporter

go 1.15

// Update percona-toolkit with `go get -v github.com/percona/percona-toolkit@3.0; go mod tidy` (without `-u`)
// until we have everything we need in a tagged release.

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/alecthomas/kong v0.2.16
	github.com/percona/exporter_shared v0.7.3-0.20210528172941-f7ec90e20757
	github.com/percona/percona-toolkit v0.0.0-20210504134948-4dd72d96b10d
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.25.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	go.mongodb.org/mongo-driver v1.5.2
)
