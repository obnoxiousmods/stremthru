package dash_api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/MunifTanjim/stremthru/internal/cache"
	newznab_indexer "github.com/MunifTanjim/stremthru/internal/newznab/indexer"
	newznab_stats "github.com/MunifTanjim/stremthru/internal/newznab/stats"
	"github.com/MunifTanjim/stremthru/internal/shared"
)

var cachedNewznabIndexerNames = cache.NewCachedValue(cache.CachedValueConfig[map[int64]string]{
	TTL: 60 * time.Second,
	Get: func() (map[int64]string, error) {
		names := map[int64]string{}
		indexers, err := newznab_indexer.GetAll()
		if err != nil {
			return names, err
		}
		for i := range indexers {
			names[indexers[i].Id] = indexers[i].Name
		}
		return names, nil
	},
})

func getNewznabIndexerNames() map[int64]string {
	names, _ := cachedNewznabIndexerNames.Get()
	return names
}

func nameForIndexerId(names map[int64]string, id int64) string {
	if n, ok := names[id]; ok && n != "" {
		return n
	}
	return "#" + strconv.FormatInt(id, 10)
}

type NewznabIndexerStatsHistoryResponse struct {
	Items []newznab_stats.AggregatedIndexerStats `json:"items"`
}

func HandleGetNewznabIndexerStatsHistory(w http.ResponseWriter, r *http.Request) {
	if !shared.IsMethod(r, http.MethodGet) {
		ErrorMethodNotAllowed(r).Send(w, r)
		return
	}

	since, _ := parseRangeSince(r)

	indexers, err := newznab_stats.GetAggregatedStats(since)
	if err != nil {
		SendError(w, r, err)
		return
	}
	if indexers == nil {
		indexers = []newznab_stats.AggregatedIndexerStats{}
	}

	names := getNewznabIndexerNames()
	for i := range indexers {
		indexers[i].IndexerName = nameForIndexerId(names, indexers[i].IndexerId)
	}

	SendData(w, r, 200, NewznabIndexerStatsHistoryResponse{Items: indexers})
}

type NewznabIndexerStatsTimeSeriesResponse struct {
	Items map[string]*newznab_stats.IndexerTimeSeries `json:"items"`
}

func HandleGetNewznabIndexerStatsTimeSeries(w http.ResponseWriter, r *http.Request) {
	if !shared.IsMethod(r, http.MethodGet) {
		ErrorMethodNotAllowed(r).Send(w, r)
		return
	}

	since, interval := parseRangeSince(r)

	indexers, err := newznab_stats.GetTimeSeriesStats(since, interval)
	if err != nil {
		SendError(w, r, err)
		return
	}
	if indexers == nil {
		indexers = map[string]*newznab_stats.IndexerTimeSeries{}
	}

	names := getNewznabIndexerNames()
	for idStr, s := range indexers {
		id, _ := strconv.ParseInt(idStr, 10, 64)
		s.Name = nameForIndexerId(names, id)
	}

	SendData(w, r, 200, NewznabIndexerStatsTimeSeriesResponse{Items: indexers})
}
