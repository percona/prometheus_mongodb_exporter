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

const contentEncodingHeader = "Content-Encoding"

// Exporter holds Exporter methods and attributes.
type Exporter struct {
	path             string
	logger           *logrus.Logger
	opts             *Opts
	client           *mongo.Client
	webListenAddress string
}

// Opts holds new exporter options.
type Opts struct {
	CompatibleMode        bool
	GlobalConnPool        bool
	URI                   string
	Path                  string
	WebListenAddress      string
	IndexStatsCollections []string
	CollStatsCollections  []string
	Logger                *logrus.Logger
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
	var client *mongo.Client
	var err error
	// Use global connection pool.
	if opts.GlobalConnPool {
		client, err = connect(ctx, opts.URI)
		if err != nil {
			return nil, err
		}
	}

	exp := &Exporter{
		client:           client,
		path:             opts.Path,
		logger:           opts.Logger,
		opts:             opts,
		webListenAddress: opts.WebListenAddress,
	}

	return exp, nil
}

func (e *Exporter) makeRegistry(ctx context.Context, client *mongo.Client) *prometheus.Registry {
	registry := prometheus.NewRegistry()
	if len(e.opts.CollStatsCollections) > 0 {
		cc := collstatsCollector{
			ctx:            ctx,
			client:         client,
			collections:    e.opts.CollStatsCollections,
			compatibleMode: e.opts.CompatibleMode,
			logger:         e.opts.Logger,
		}
		registry.MustRegister(&cc)
	}

	if len(e.opts.IndexStatsCollections) > 0 {
		ic := indexstatsCollector{
			ctx:         ctx,
			client:      client,
			collections: e.opts.IndexStatsCollections,
			logger:      e.opts.Logger,
		}
		registry.MustRegister(&ic)
	}

	ddc := diagnosticDataCollector{
		ctx:            ctx,
		client:         client,
		compatibleMode: e.opts.CompatibleMode,
		logger:         e.opts.Logger,
	}
	registry.MustRegister(&ddc)

	rsgsc := replSetGetStatusCollector{
		ctx:            ctx,
		client:         client,
		compatibleMode: e.opts.CompatibleMode,
		logger:         e.opts.Logger,
	}
	registry.MustRegister(&rsgsc)

	return registry
}

// Run starts the exporter.
func (e *Exporter) Run() {
	handler := func() http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Use per-request connection.
			if !e.opts.GlobalConnPool {
				var err error
				e.client, err = connect(ctx, e.opts.URI)
				if err != nil {
					e.logger.Errorf("Cannot connect to MongoDB: %v", err)
					w.Header().Del(contentEncodingHeader)
					http.Error(
						w,
						"An error has occurred while connecting to MongoDB:\n\n"+err.Error(),
						http.StatusInternalServerError,
					)
				}

				defer func() {
					if err = e.client.Disconnect(ctx); err != nil {
						e.logger.Errorf("Cannot disconnect mongo client: %v", err)
					}
				}()
			}

			registry := e.makeRegistry(ctx, e.client)

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
	}()

	exporter_shared.RunServer("MongoDB", e.webListenAddress, e.path, handler)
}

func connect(ctx context.Context, dsn string) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(dsn)
	clientOpts.SetDirect(true)
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
