# MongoDB exporter
[![Release](https://img.shields.io/github/release/percona/mongodb_exporter.svg?style=flat)](https://github.com/percona/mongodb_exporter/releases/latest)
[![Build Status](https://github.com/percona/mongodb_exporter/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/percona/mongodb_exporter/actions/workflows/go.yml?query=branch%3Amain)
[![codecov.io Code Coverage](https://img.shields.io/codecov/c/github/percona/mongodb_exporter.svg?maxAge=2592000)](https://codecov.io/github/percona/mongodb_exporter?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/percona/mongodb_exporter)](https://goreportcard.com/report/github.com/percona/mongodb_exporter)
[![CLA assistant](https://cla-assistant.percona.com/readme/badge/percona/mongodb_exporter)](https://cla-assistant.percona.com/percona/mongodb_exporter)
[![Discord](https://img.shields.io/discord/808660945513611334?style=flat)](http://per.co.na/discord)

This is the new MongoDB exporter implementation that handles ALL metrics exposed by MongoDB monitoring commands.
This new implementation loops over all the fields exposed in diagnostic commands and tries to get data from them.

Currently, these metric sources are implemented:
- $collStats
- $indexStats
- getDiagnosticData
- replSetGetStatus
- serverStatus

| Old Percona MongoDB exporter                                                  |
|:------------------------------------------------------------------------------|
| old 0.1x.y version (ex `master` branch) is moved to the `release-0.1x` branch.|
| If you considering migrating from the old version of the exporter  - you can  |
| use flag `--compatible-mode` to expose metrics in the old metric names. This  |
| will simplify migration to the new version for you.                           |


## Flags
|Flag|Description|Example|
|-----|-----|-----|
|-h, \-\-help|Show context-sensitive help||
|\-\-compatible-mode|Exposes new metrics in the new and old format at the same time||
|\-\-discovering-mode|Enable autodiscover collections from databases which set in collstats-colls and indexstats-colls||
|\-\-mongodb.collstats-colls|List of comma separated databases.collections to get stats|\-\-mongodb.collstats-colls=testdb.testcol1,testdb.testcol2|
|\-\-mongodb.direct-connect|Whether or not a direct connect should be made. Direct connections are not valid if multiple hosts are specified or an SRV URI is used|\-\-mongodb.direct-connect=false|
|\-\-mongodb.indexstats-colls|List of comma separated database.collections to get index stats|\-\-mongodb.indexstats-colls=db1.col1,db1.col2|
|\-\-mongodb.uri|MongoDB connection URI ($MONGODB_URI)|\-\-mongodb.uri=mongodb://user:pass@127.0.0.1:27017/admin?ssl=true|
|\-\-mongodb.global-conn-pool|Use global connection pool instead of creating new connection for each http request.||
|\-\-web.listen-address|Address to listen on for web interface and telemetry|\-\-web.listen-address=":9216"|
|\-\-web.telemetry-path|Metrics expose path|\-\-web.telemetry-path="/metrics"|
|\-\-log.level|Only log messages with the given severity or above. Valid levels: [debug, info, warn, error]|\-\-log.level="error"|
|\-\-disable.diagnosticdata|Disable collecting metrics from getDiagnosticData||
|\-\-disable.replicasetstatus|Disable collecting metrics from replSetGetStatus||
|\-\-disable.replicasetconfig|Disable collecting metrics from replSetGetConfig||
|--version|Show version and exit|

 ### Build the exporter
The build process uses the dockerized version of goreleaser so you don't need to install Go.
Just run `make release` and the new binaries will be generated under the build directory.
```
├── build
│ ├── config.yaml
│ ├── mongodb_exporter_7c73946_checksums.txt
│ ├── mongodb_exporter-7c73946.darwin-amd64.tar.gz
│ ├── mongodb_exporter-7c73946.linux-amd64.tar.gz
│ ├── mongodb_exporter_darwin_amd64
│ │ └── mongodb_exporter <--- MacOS binary
│ └── mongodb_exporter_linux_amd64
│ └── mongodb_exporter <--- Linux binary
```
### Running the exporter
If you built the exporter using the method mentioned in the previous section, the generated binaries are in `mongodb_exporter_linux_amd64/mongodb_exporter` or `mongodb_exporter_darwin_amd64/mongodb_exporter`

#### Example
```
mongodb_exporter_linux_amd64/mongodb_exporter --mongodb.uri=mongodb://127.0.0.1:17001
```
#### Enabling collstats metrics gathering
`--mongodb.collstats-colls` receives a list of databases and collections to monitor using collstats.
Usage example: `--mongodb.collstats-colls=database1.collection1,database2.collection2`
```
mongodb_exporter_linux_amd64/mongodb_exporter --mongodb.uri=mongodb://127.0.0.1:17001 --mongodb.collstats-colls=db1.c1,db2.c2
```
#### Enabling compatibility mode.
When compatibility mode is enabled by the `--compatible-mode`, the exporter will expose all new metrics with the new naming and labeling schema and at the same time will expose metrics in the version 1 compatible way.
For example, if compatibility mode is enabled, the metric `mongodb_ss_wt_log_log_bytes_written` (new format)
```
# HELP mongodb_ss_wt_log_log_bytes_written serverStatus.wiredTiger.log.
# TYPE mongodb_ss_wt_log_log_bytes_written untyped
mongodb_ss_wt_log_log_bytes_written 2.6208e+06
```
will be also exposed as `mongodb_mongod_wiredtiger_log_bytes_total`  with the `unwritten` label.
```
HELP mongodb_mongod_wiredtiger_log_bytes_total mongodb_mongod_wiredtiger_log_bytes_total
# TYPE mongodb_mongod_wiredtiger_log_bytes_total untyped
mongodb_mongod_wiredtiger_log_bytes_total{type="unwritten"} 2.6208e+06
```

## Submitting Bug Reports and adding new functionality

please see [Contribution Guide](CONTRIBUTING.md)
