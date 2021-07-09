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

package exporter

import (
	"context"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// const (
// 	replicationNotEnabled        = 76
// 	replicationNotYetInitialized = 94
// )

type replSetGetConfigCollector struct {
	ctx            context.Context
	client         *mongo.Client
	compatibleMode bool
	logger         *logrus.Logger
	topologyInfo   labelsGetter
}

func (d *replSetGetConfigCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(d, ch)
}

func (d *replSetGetConfigCollector) Collect(ch chan<- prometheus.Metric) {
	cmd := bson.D{{Key: "replSetGetConfig", Value: "1"}}
	res := d.client.Database("admin").RunCommand(d.ctx, cmd)

	var m bson.M

	if err := res.Decode(&m); err != nil {
		if e, ok := err.(mongo.CommandError); ok {
			if e.Code == replicationNotYetInitialized || e.Code == replicationNotEnabled {
				return
			}
		}
		d.logger.Errorf("cannot get replSetGetConfig: %s", err)

		return
	}

	config, ok := m["config"].(bson.M)
	if !ok {
		err := errors.Wrapf(errUnexpectedDataType, "%T for data field", m["config"])
		d.logger.Errorf("cannot decode getDiagnosticData: %s", err)

		return
	}
	m = config

	d.logger.Debug("replSetGetConfig result:")
	debugResult(d.logger, m)

	for _, metric := range makeMetrics("cfg", m, d.topologyInfo.baseLabels(), d.compatibleMode) {
		ch <- metric
	}
}

var _ prometheus.Collector = (*replSetGetConfigCollector)(nil)
