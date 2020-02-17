package mongod

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/percona/mongodb_exporter/collector/common"
)

var (
	collectionSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "size",
		Help:      "The total size in memory of all records in a collection",
	}, []string{"db", "coll"})
	collectionObjectCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "count",
		Help:      "The number of objects or documents in this collection",
	}, []string{"db", "coll"})
	collectionAvgObjSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "avgobjsize",
		Help:      "The average size of an object in the collection (plus any padding)",
	}, []string{"db", "coll"})
	collectionStorageSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "storage_size",
		Help:      "The total amount of storage allocated to this collection for document storage",
	}, []string{"db", "coll"})
	collectionIndexes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "indexes",
		Help:      "The number of indexes on the collection",
	}, []string{"db", "coll"})
	collectionIndexesSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "indexes_size",
		Help:      "The total size of all indexes",
	}, []string{"db", "coll"})
	collectionIndexSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "index_size",
		Help:      "The individual index size",
	}, []string{"db", "coll", "index"})
)

// CollectionStatList contains stats from all collections
type CollectionStatList struct {
	Members []CollectionStatus
}

// CollectionStatus represents stats about a collection in database (mongod and raw from mongos)
type CollectionStatus struct {
	Database    string
	Name        string
	Size        int                `bson:"size,omitempty"`
	Count       int                `bson:"count,omitempty"`
	AvgObjSize  int                `bson:"avgObjSize,omitempty"`
	StorageSize int                `bson:"storageSize,omitempty"`
	IndexesSize int                `bson:"totalIndexSize,omitempty"`
	IndexSizes  map[string]float64 `bson:"indexSizes,omitempty"`
}

// Export exports database stats to prometheus
func (collStatList *CollectionStatList) Export(ch chan<- prometheus.Metric) {
	// reset previously collected values
	collectionSize.Reset()
	collectionObjectCount.Reset()
	collectionAvgObjSize.Reset()
	collectionStorageSize.Reset()
	collectionIndexes.Reset()
	collectionIndexesSize.Reset()
	collectionIndexSize.Reset()
	for _, member := range collStatList.Members {
		ls := prometheus.Labels{
			"db":   member.Database,
			"coll": member.Name,
		}
		collectionSize.With(ls).Set(float64(member.Size))
		collectionObjectCount.With(ls).Set(float64(member.Count))
		collectionAvgObjSize.With(ls).Set(float64(member.AvgObjSize))
		collectionStorageSize.With(ls).Set(float64(member.StorageSize))
		collectionIndexes.With(ls).Set(float64(len(member.IndexSizes)))
		collectionIndexesSize.With(ls).Set(float64(member.IndexesSize))
		for indexName, size := range member.IndexSizes {
			ls = prometheus.Labels{
				"db":    member.Database,
				"coll":  member.Name,
				"index": indexName,
			}
			collectionIndexSize.With(ls).Set(size)
		}
	}
	collectionSize.Collect(ch)
	collectionObjectCount.Collect(ch)
	collectionAvgObjSize.Collect(ch)
	collectionStorageSize.Collect(ch)
	collectionIndexes.Collect(ch)
	collectionIndexesSize.Collect(ch)
	collectionIndexSize.Collect(ch)
}

// Describe describes database stats for prometheus
func (collStatList *CollectionStatList) Describe(ch chan<- *prometheus.Desc) {
	collectionSize.Describe(ch)
	collectionObjectCount.Describe(ch)
	collectionAvgObjSize.Describe(ch)
	collectionStorageSize.Describe(ch)
	collectionIndexes.Describe(ch)
	collectionIndexesSize.Describe(ch)
}

var logSuppressCS = make(map[string]struct{})

const keyCS = ""

// GetCollectionStatList returns stats for all non-system collections
func GetCollectionStatList(client *mongo.Client) *CollectionStatList {
	collectionStatList := &CollectionStatList{}
	dbNames, err := client.ListDatabaseNames(context.TODO(), bson.M{})
	if err != nil {
		if _, ok := logSuppressCS[keyCS]; !ok {
			log.Warnf("%s. Collection stats will not be collected. This log message will be suppressed from now.", err)
			logSuppressCS[keyCS] = struct{}{}
		}
		return nil
	}

	delete(logSuppressCS, keyCS)
	for _, dbName := range dbNames {
		if common.IsSystemDB(dbName) {
			continue
		}

		collNames, err := client.Database(dbName).ListCollectionNames(context.TODO(), bson.M{})
		if err != nil {
			if _, ok := logSuppressCS[dbName]; !ok {
				log.Warnf("%s. Collection stats will not be collected for this db. This log message will be suppressed from now.", err)
				logSuppressCS[dbName] = struct{}{}
			}
			continue
		}

		delete(logSuppressCS, dbName)
		for _, collName := range collNames {
			if common.IsSystemCollection(collName) {
				continue
			}

			fullCollName := common.CollFullName(dbName, collName)
			collStatus := CollectionStatus{}
			err = client.Database(dbName).RunCommand(context.TODO(), bson.D{{"collStats", collName}, {"scale", 1}}).Decode(&collStatus)
			if err != nil {
				if _, ok := logSuppressCS[fullCollName]; !ok {
					log.Warnf("%s. Collection stats will not be collected for this collection. This log message will be suppressed from now.", err)
					logSuppressCS[fullCollName] = struct{}{}
				}
				continue
			}

			delete(logSuppressCS, fullCollName)
			collStatus.Database = dbName
			collStatus.Name = collName
			collectionStatList.Members = append(collectionStatList.Members, collStatus)
		}
	}

	return collectionStatList
}
