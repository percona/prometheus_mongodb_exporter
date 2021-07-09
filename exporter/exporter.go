// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Package exporter implements the collectors and metrics handlers.
package exporter

import (
	"context"
	"fmt"
	"net/http"

	"github.com/percona/exporter_shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Exporter holds Exporter methods and attributes.
type Exporter struct {
	path             string
	client           *mongo.Client
	logger           *logrus.Logger
	opts             *Opts
	webListenAddress string
	topologyInfo     labelsGetter
}

// Opts holds new exporter options.
type Opts struct {
	CompatibleMode          bool
	DiscoveringMode         bool
	GlobalConnPool          bool
	DirectConnect           bool
	URI                     string
	Path                    string
	WebListenAddress        string
	IndexStatsCollections   []string
	CollStatsCollections    []string
	Logger                  *logrus.Logger
	DisableDiagnosticData   bool
	DisableReplicasetStatus bool
	DisableReplicasetConfig bool
}

var (
	errCannotHandleType   = fmt.Errorf("don't know how to handle data type")
	errUnexpectedDataType = fmt.Errorf("unexpected data type")
)

// New connects to the database and returns a new Exporter instance.
func New(opts *Opts) (*Exporter, error) {
	if opts == nil {
		opts = new(Opts)
	}

	if opts.Logger == nil {
		opts.Logger = logrus.New()
	}

	ctx := context.Background()

	exp := &Exporter{
		path:             opts.Path,
		logger:           opts.Logger,
		opts:             opts,
		webListenAddress: opts.WebListenAddress,
	}
	if opts.GlobalConnPool {
		var err error
		exp.client, err = connect(ctx, opts.URI, opts.DirectConnect)
		if err != nil {
			return nil, err
		}

		exp.topologyInfo, err = newTopologyInfo(ctx, exp.client)
		if err != nil {
			return nil, err
		}
	}

	return exp, nil
}

func (e *Exporter) makeRegistry(ctx context.Context, client *mongo.Client, topologyInfo labelsGetter) *prometheus.Registry {
	// TODO: use NewPedanticRegistry when mongodb_exporter code fulfils its requirements (https://jira.percona.com/browse/PMM-6630).
	registry := prometheus.NewRegistry()

	gc := generalCollector{
		ctx:    ctx,
		client: client,
		logger: e.opts.Logger,
	}
	registry.MustRegister(&gc)

	nodeType, err := getNodeType(ctx, client)
	if err != nil {
		e.logger.Errorf("Cannot get node type to check if this is a mongos: %s", err)
	}

	if len(e.opts.CollStatsCollections) > 0 {
		cc := collstatsCollector{
			ctx:             ctx,
			client:          client,
			collections:     e.opts.CollStatsCollections,
			compatibleMode:  e.opts.CompatibleMode,
			discoveringMode: e.opts.DiscoveringMode,
			logger:          e.opts.Logger,
			topologyInfo:    topologyInfo,
		}
		registry.MustRegister(&cc)
	}

	if len(e.opts.IndexStatsCollections) > 0 {
		ic := indexstatsCollector{
			ctx:             ctx,
			client:          client,
			collections:     e.opts.IndexStatsCollections,
			discoveringMode: e.opts.DiscoveringMode,
			logger:          e.opts.Logger,
			topologyInfo:    topologyInfo,
		}
		registry.MustRegister(&ic)
	}

	if !e.opts.DisableDiagnosticData {
		ddc := diagnosticDataCollector{
			ctx:            ctx,
			client:         client,
			compatibleMode: e.opts.CompatibleMode,
			logger:         e.opts.Logger,
			topologyInfo:   topologyInfo,
		}
		registry.MustRegister(&ddc)
	}

	// replSetGetStatus is not supported through mongos
	if !e.opts.DisableReplicasetStatus && nodeType != typeMongos {
		rsgsc := replSetGetStatusCollector{
			ctx:            ctx,
			client:         client,
			compatibleMode: e.opts.CompatibleMode,
			logger:         e.opts.Logger,
			topologyInfo:   topologyInfo,
		}
		registry.MustRegister(&rsgsc)
	}

	if !e.opts.DisableReplicasetConfig {
		rsgcc := replSetGetConfigCollector{
			ctx:            ctx,
			client:         client,
			compatibleMode: e.opts.CompatibleMode,
			logger:         e.opts.Logger,
			topologyInfo:   topologyInfo,
		}
		registry.MustRegister(&rsgcc)
	}

	return registry
}

func (e *Exporter) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		client := e.client
		topologyInfo := e.topologyInfo
		// Use per-request connection.
		if !e.opts.GlobalConnPool {
			var err error
			client, err = connect(ctx, e.opts.URI, e.opts.DirectConnect)
			if err != nil {
				e.logger.Errorf("Cannot connect to MongoDB: %v", err)
				http.Error(
					w,
					"An error has occurred while connecting to MongoDB:\n\n"+err.Error(),
					http.StatusInternalServerError,
				)

				return
			}

			defer func() {
				if err = client.Disconnect(ctx); err != nil {
					e.logger.Errorf("Cannot disconnect mongo client: %v", err)
				}
			}()

			topologyInfo, err = newTopologyInfo(ctx, client)
			if err != nil {
				e.logger.Errorf("Cannot get topology info: %v", err)
				http.Error(
					w,
					"An error has occurred while getting topology info:\n\n"+err.Error(),
					http.StatusInternalServerError,
				)

				return
			}
		}

		registry := e.makeRegistry(ctx, client, topologyInfo)

		gatherers := prometheus.Gatherers{}
		gatherers = append(gatherers, prometheus.DefaultGatherer)
		gatherers = append(gatherers, registry)

		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
			ErrorLog:      e.logger,
		})

		h.ServeHTTP(w, r)
	})
}

// Run starts the exporter.
func (e *Exporter) Run() {
	handler := e.handler()
	exporter_shared.RunServer("MongoDB", e.webListenAddress, e.path, handler)
}

func connect(ctx context.Context, dsn string, directConnect bool) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(dsn)
	clientOpts.SetDirect(directConnect)
	clientOpts.SetAppName("mongodb_exporter")

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}

	if err = client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client, nil
}
