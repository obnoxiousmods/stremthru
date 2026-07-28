package newznab_stats

import (
	"database/sql"
	"fmt"
	"math"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/MunifTanjim/stremthru/internal/db"
	"github.com/MunifTanjim/stremthru/internal/logger"
	"github.com/MunifTanjim/stremthru/internal/util"
	"github.com/rs/xid"
)

const TableName = "newznab_indexer_stats"

var log = logger.Scoped("newznab/stats")

type NewznabIndexerStat struct {
	Id          string
	IndexerId   int64
	Operation   string
	ErrorType   string
	Error       sql.NullString
	HTTPStatus  int
	LatencyMs   float64
	ResultCount int64
	Bytes       int64
	CAt         db.Timestamp
}

var Column = struct {
	Id          string
	IndexerId   string
	Operation   string
	ErrorType   string
	Error       string
	HTTPStatus  string
	LatencyMs   string
	ResultCount string
	Bytes       string
	CAt         string
}{
	Id:          "id",
	IndexerId:   "indexer_id",
	Operation:   "operation",
	ErrorType:   "error_type",
	Error:       "error",
	HTTPStatus:  "http_status",
	LatencyMs:   "latency_ms",
	ResultCount: "result_count",
	Bytes:       "bytes",
	CAt:         "cat",
}

var insertColumns = db.JoinColumnNames(
	Column.Id,
	Column.IndexerId,
	Column.Operation,
	Column.ErrorType,
	Column.Error,
	Column.HTTPStatus,
	Column.LatencyMs,
	Column.ResultCount,
	Column.Bytes,
)

var query_insert_batch = fmt.Sprintf(
	`INSERT INTO %s (%s) VALUES `,
	TableName, insertColumns,
)

var query_insert_batch_values_placeholder = "(" + util.RepeatJoin("?", 9, ",") + ")"

func InsertBatch(stats []NewznabIndexerStat) error {
	if len(stats) == 0 {
		return nil
	}
	query := query_insert_batch + util.RepeatJoin(query_insert_batch_values_placeholder, len(stats), ",")
	args := make([]any, 0, len(stats)*9)
	for i := range stats {
		s := &stats[i]
		if s.Id == "" {
			s.Id = xid.New().String()
		}
		args = append(args,
			s.Id, s.IndexerId, s.Operation, s.ErrorType, s.Error,
			s.HTTPStatus, s.LatencyMs, s.ResultCount, s.Bytes,
		)
	}
	_, err := db.Exec(query, args...)
	return err
}

type AggregatedIndexerStats struct {
	IndexerId            int64            `json:"indexer_id"`
	IndexerName          string           `json:"indexer_name"`
	SearchCount          int64            `json:"search_count"`
	SearchOk             int64            `json:"search_ok"`
	SearchError          int64            `json:"search_error"`
	ZeroResultCount      int64            `json:"zero_result_count"`
	ResultTotal          int64            `json:"result_total"`
	AvgResultCount       float64          `json:"avg_result_count"`
	SearchBytes          int64            `json:"search_bytes"`
	AvgSearchLatencyMs   float64          `json:"avg_search_latency_ms"`
	P50SearchLatencyMs   float64          `json:"p50_search_latency_ms"`
	P95SearchLatencyMs   float64          `json:"p95_search_latency_ms"`
	P99SearchLatencyMs   float64          `json:"p99_search_latency_ms"`
	DownloadCount        int64            `json:"download_count"`
	DownloadBytes        int64            `json:"download_bytes"`
	DownloadErrorCount   int64            `json:"download_error_count"`
	AvgDownloadLatencyMs float64          `json:"avg_download_latency_ms"`
	ErrorsByType         map[string]int64 `json:"errors_by_type"`
	SuccessRate          float64          `json:"success_rate"`
	ZeroResultRate       float64          `json:"zero_result_rate"`
	LastSeenAt           string           `json:"last_seen_at"`
}

var query_get_aggregated_stats = fmt.Sprintf(
	`SELECT %s, %s, %s, %s, %s, %s, %s FROM %s WHERE %s >= ?`,
	Column.IndexerId, Column.Operation, Column.ErrorType,
	Column.LatencyMs, Column.ResultCount, Column.Bytes, Column.CAt,
	TableName, Column.CAt,
)

func GetAggregatedStats(since time.Time) ([]AggregatedIndexerStats, error) {
	rows, err := db.Query(query_get_aggregated_stats, db.Timestamp{Time: since})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type accumulator struct {
		stats             AggregatedIndexerStats
		searchLatencies   []float64
		downloadLatencies []float64
		resultTotal       int64
		resultObsCount    int64
		lastSeen          time.Time
	}
	accByIndexer := make(map[int64]*accumulator)

	for rows.Next() {
		var indexerId int64
		var operation, errorType string
		var latencyMs float64
		var resultCount, bytes int64
		var cat db.Timestamp
		if err := rows.Scan(&indexerId, &operation, &errorType, &latencyMs, &resultCount, &bytes, &cat); err != nil {
			return nil, err
		}

		acc, ok := accByIndexer[indexerId]
		if !ok {
			acc = &accumulator{
				stats: AggregatedIndexerStats{
					IndexerId:    indexerId,
					ErrorsByType: make(map[string]int64),
				},
			}
			accByIndexer[indexerId] = acc
		}
		if cat.Time.After(acc.lastSeen) {
			acc.lastSeen = cat.Time
		}

		switch operation {
		case string(OperationSearch):
			if errorType == string(ErrorTypeRateLimit) {
				acc.stats.ErrorsByType[errorType]++
				break
			}
			acc.stats.SearchCount++
			if errorType == "" {
				acc.stats.SearchOk++
				acc.stats.SearchBytes += bytes
				acc.searchLatencies = append(acc.searchLatencies, latencyMs)
				if resultCount == 0 {
					acc.stats.ZeroResultCount++
				}
				acc.resultTotal += resultCount
				acc.resultObsCount++
			} else {
				acc.stats.SearchError++
				acc.stats.ErrorsByType[errorType]++
			}
		case string(OperationDownload):
			if errorType == string(ErrorTypeRateLimit) {
				acc.stats.ErrorsByType[errorType]++
				break
			}
			acc.stats.DownloadCount++
			if errorType == "" {
				acc.stats.DownloadBytes += bytes
				acc.downloadLatencies = append(acc.downloadLatencies, latencyMs)
			} else {
				acc.stats.DownloadErrorCount++
				acc.stats.ErrorsByType[errorType]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	indexerIds := make([]int64, 0, len(accByIndexer))
	for id := range accByIndexer {
		indexerIds = append(indexerIds, id)
	}
	slices.Sort(indexerIds)

	result := make([]AggregatedIndexerStats, 0, len(indexerIds))
	for _, id := range indexerIds {
		acc := accByIndexer[id]
		s := &acc.stats

		if len(acc.searchLatencies) > 0 {
			slices.Sort(acc.searchLatencies)
			var total float64
			for _, v := range acc.searchLatencies {
				total += v
			}
			s.AvgSearchLatencyMs = round2(total / float64(len(acc.searchLatencies)))
			s.P50SearchLatencyMs = round2(util.Percentile(acc.searchLatencies, 50))
			s.P95SearchLatencyMs = round2(util.Percentile(acc.searchLatencies, 95))
			s.P99SearchLatencyMs = round2(util.Percentile(acc.searchLatencies, 99))
		}
		if len(acc.downloadLatencies) > 0 {
			var total float64
			for _, v := range acc.downloadLatencies {
				total += v
			}
			s.AvgDownloadLatencyMs = round2(total / float64(len(acc.downloadLatencies)))
		}
		s.ResultTotal = acc.resultTotal
		if acc.resultObsCount > 0 {
			s.AvgResultCount = round2(float64(acc.resultTotal) / float64(acc.resultObsCount))
		}
		if s.SearchCount > 0 {
			s.SuccessRate = round2(float64(s.SearchOk) / float64(s.SearchCount) * 100)
			s.ZeroResultRate = round2(float64(s.ZeroResultCount) / float64(s.SearchCount) * 100)
		}
		if !acc.lastSeen.IsZero() {
			s.LastSeenAt = acc.lastSeen.UTC().Format(time.RFC3339)
		}
		result = append(result, *s)
	}
	return result, nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

type TimeSeriesBucket struct {
	Time          string  `json:"time"`
	SearchCount   int64   `json:"search_count"`
	SearchError   int64   `json:"search_error"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	DownloadCount int64   `json:"download_count"`
	DownloadBytes int64   `json:"download_bytes"`
}

type IndexerTimeSeries struct {
	Name    string             `json:"name"`
	Buckets []TimeSeriesBucket `json:"buckets"`
}

var query_get_time_series_stats = fmt.Sprintf(
	`SELECT %s, %%s AS bucket, %s, %s, %s, %s FROM %s WHERE %s >= ? ORDER BY %s, bucket`,
	Column.IndexerId,
	Column.Operation, Column.ErrorType, Column.LatencyMs, Column.Bytes,
	TableName, Column.CAt, Column.IndexerId,
)

func GetTimeSeriesStats(since time.Time, interval string) (map[string]*IndexerTimeSeries, error) {
	bucketExpr, err := db.TimeBucketExpr(Column.CAt, interval)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(query_get_time_series_stats, bucketExpr)
	rows, err := db.Query(query, db.Timestamp{Time: since})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type bucketKey struct {
		IndexerId int64
		Bucket    string
	}
	type accumulator struct {
		bucket    TimeSeriesBucket
		latencies []float64
	}

	accByKey := make(map[bucketKey]*accumulator)
	var keyOrder []bucketKey

	for rows.Next() {
		var indexerId int64
		var bucket, operation, errorType string
		var latencyMs float64
		var bytes int64
		if err := rows.Scan(&indexerId, &bucket, &operation, &errorType, &latencyMs, &bytes); err != nil {
			return nil, err
		}

		if errorType == string(ErrorTypeRateLimit) {
			continue
		}

		key := bucketKey{IndexerId: indexerId, Bucket: bucket}
		acc, ok := accByKey[key]
		if !ok {
			acc = &accumulator{bucket: TimeSeriesBucket{Time: bucket}}
			accByKey[key] = acc
			keyOrder = append(keyOrder, key)
		}

		switch operation {
		case string(OperationSearch):
			acc.bucket.SearchCount++
			if errorType == "" {
				acc.latencies = append(acc.latencies, latencyMs)
			} else {
				acc.bucket.SearchError++
			}
		case string(OperationDownload):
			acc.bucket.DownloadCount++
			if errorType == "" {
				acc.bucket.DownloadBytes += bytes
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make(map[string]*IndexerTimeSeries)
	for _, key := range keyOrder {
		acc := accByKey[key]
		if len(acc.latencies) > 0 {
			var total float64
			for _, v := range acc.latencies {
				total += v
			}
			acc.bucket.AvgLatencyMs = round2(total / float64(len(acc.latencies)))
		}
		idStr := strconv.FormatInt(key.IndexerId, 10)
		ts, ok := result[idStr]
		if !ok {
			ts = &IndexerTimeSeries{}
			result[idStr] = ts
		}
		ts.Buckets = append(ts.Buckets, acc.bucket)
	}
	return result, nil
}

var query_delete_older_than = fmt.Sprintf(
	`DELETE FROM %s WHERE %s < ?`,
	TableName, Column.CAt,
)

func DeleteOlderThan(t time.Time) (int64, error) {
	res, err := db.Exec(query_delete_older_than, db.Timestamp{Time: t})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

var backgroundJobQuit chan struct{}

func CleanupBackgroundJob() {
	shuttingDown.Store(true)
	if backgroundJobQuit != nil {
		close(backgroundJobQuit)
		backgroundJobQuit = nil
		<-writerDone
	}
}

func initBackgroundJob() {
	backgroundJobQuit = make(chan struct{})
	startWriter()
	go func() {
		cleanupTicker := time.NewTicker(1 * time.Hour)
		defer cleanupTicker.Stop()

		for {
			select {
			case <-backgroundJobQuit:
				return
			case <-cleanupTicker.C:
				cutoff := time.Now().AddDate(0, 0, -90)
				if count, err := DeleteOlderThan(cutoff); err != nil {
					log.Error("failed to cleanup old newznab indexer stats", "error", err)
				} else if count > 0 {
					log.Info("cleaned up old newznab indexer stats", "count", count)
				}
			}
		}
	}()
}

var initBackgroundJobOnce sync.Once

func InitBackgroundJob() {
	initBackgroundJobOnce.Do(initBackgroundJob)
}
