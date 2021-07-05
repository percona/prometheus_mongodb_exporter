package mongod

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	memberHiddenDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, subsystem, "member_hidden"),
		"This field conveys if the member is hidden (1) or not-hidden (0).",
		[]string{"id", "host"},
		nil,
	)

	memberArbiterDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, subsystem, "member_arbiter"),
		"This field conveys if the member is an arbiter (1) or not (0).",
		[]string{"id", "host"},
		nil,
	)

	memberBuildIndexesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, subsystem, "member_build_indexes"),
		"This field conveys if the member is  builds indexes (1) or not (0).",
		[]string{"id", "host"},
		nil,
	)

	memberPriorityDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, subsystem, "member_priority"),
		"This field conveys the priority of a given member",
		[]string{"id", "host"},
		nil,
	)

	memberVotesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, subsystem, "member_votes"),
		"This field conveys the number of votes of a given member",
		[]string{"id", "host"},
		nil,
	)
)

// OuterReplSetConf Although the docs say that it returns a map with id etc. it *actually* returns that wrapped in a map.
type OuterReplSetConf struct {
	Config ReplSetConf `bson:"config"`
}

// ReplSetConf keeps the data returned by the GetReplSetConf method
type ReplSetConf struct {
	ID      string       `bson:"_id"`
	Version int          `bson:"version"`
	Members []MemberConf `bson:"members"`
}

// MemberConf represents an array element of ReplSetConf.Members
type MemberConf struct {
	ID           int32   `bson:"_id"`
	Host         string  `bson:"host"`
	ArbiterOnly  bool    `bson:"arbiterOnly"`
	BuildIndexes bool    `bson:"buildIndexes"`
	Hidden       bool    `bson:"hidden"`
	Priority     float64 `bson:"priority"`

	Tags       map[string]string `bson:"tags"`
	SlaveDelay float64           `bson:"slaveDelay"`
	Votes      int32             `bson:"votes"`
}

// Export exports the replSetGetStatus stati to be consumed by prometheus
func (replConf *ReplSetConf) Export(ch chan<- prometheus.Metric) {
	for _, member := range replConf.Members {
		if member.Hidden {
			ch <- prometheus.MustNewConstMetric(memberHiddenDesc, prometheus.GaugeValue, 1, replConf.ID, member.Host)
		} else {
			ch <- prometheus.MustNewConstMetric(memberHiddenDesc, prometheus.GaugeValue, 0, replConf.ID, member.Host)
		}

		if member.ArbiterOnly {
			ch <- prometheus.MustNewConstMetric(memberArbiterDesc, prometheus.GaugeValue, 1, replConf.ID, member.Host)
		} else {
			ch <- prometheus.MustNewConstMetric(memberArbiterDesc, prometheus.GaugeValue, 0, replConf.ID, member.Host)
		}

		if member.BuildIndexes {
			ch <- prometheus.MustNewConstMetric(memberBuildIndexesDesc, prometheus.GaugeValue, 1, replConf.ID, member.Host)
		} else {
			ch <- prometheus.MustNewConstMetric(memberBuildIndexesDesc, prometheus.GaugeValue, 0, replConf.ID, member.Host)
		}

		ch <- prometheus.MustNewConstMetric(memberPriorityDesc, prometheus.GaugeValue, float64(member.Priority), replConf.ID, member.Host)
		ch <- prometheus.MustNewConstMetric(memberVotesDesc, prometheus.GaugeValue, float64(member.Votes), replConf.ID, member.Host)
	}
}

// Describe describes the replSetGetStatus metrics for prometheus
func (replConf *ReplSetConf) Describe(ch chan<- *prometheus.Desc) {
	ch <- memberHiddenDesc
	ch <- memberArbiterDesc
	ch <- memberBuildIndexesDesc
	ch <- memberPriorityDesc
	ch <- memberVotesDesc
}

// GetReplSetConf returns the replica status info
func GetReplSetConf(client *mongo.Client) *ReplSetConf {
	result := &OuterReplSetConf{}
	err := client.Database("admin").RunCommand(context.TODO(), bson.D{{"replSetGetConfig", 1}}).Decode(result)
	if err != nil {
		log.Errorf("Failed to get replSetGetConfig: %s.", err)
		return nil
	}
	return &result.Config
}
