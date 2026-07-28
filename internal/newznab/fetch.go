package newznab

import (
	"database/sql"
	"errors"
	"net/url"
	"time"

	"github.com/MunifTanjim/stremthru/internal/logger"
	newznab_indexer "github.com/MunifTanjim/stremthru/internal/newznab/indexer"
	newznab_stats "github.com/MunifTanjim/stremthru/internal/newznab/stats"
	"github.com/MunifTanjim/stremthru/internal/usenet/nzb_info"
)

// FetchNZBFromInfo re-fetches an NZB file using the indexer associated with the
// given nzb_info row. If indexer_id is not set, it tries to infer from the URL
// hostname and lazy-fills the row when there is exactly one matching indexer.
// When an indexer is associated, it applies rate limiting, refreshes the apikey
// query parameter using the indexer's current credentials, and records download
// stats. Falls back to a plain fetch if no indexer can be associated.
func FetchNZBFromInfo(info *nzb_info.NZBInfo, log *logger.Logger) (*nzb_info.NZBFile, error) {
	indexerId := info.IndexerId.Int64
	if !info.IndexerId.Valid {
		if resolved := newznab_indexer.ResolveIdByURL(info.URL); resolved != 0 {
			indexerId = resolved
			info.IndexerId = sql.NullInt64{Int64: indexerId, Valid: true}
			if err := nzb_info.SetIndexerId(info.Id, indexerId); err != nil && log != nil {
				log.Warn("failed to set indexer_id on nzb_info", "error", err, "id", info.Id)
			}
		}
	}

	if indexerId == 0 {
		return nzb_info.FetchNZBFile(info.URL, info.Name, log)
	}

	indexer, err := newznab_indexer.GetById(indexerId)
	if err != nil || indexer == nil {
		return nzb_info.FetchNZBFile(info.URL, info.Name, log)
	}

	rl, err := indexer.GetRateLimiter()
	if err != nil {
		newznab_stats.RecordDownload(indexerId, 0, 0, err)
		return nil, err
	}
	if rl != nil {
		result, err := rl.Try()
		if err != nil {
			newznab_stats.RecordDownload(indexerId, 0, 0, err)
			return nil, err
		}
		if !result.Allowed {
			newznab_stats.RecordRateLimited(indexerId, newznab_stats.OperationDownload)
			return nil, errors.New("rate limit exceeded")
		}
	}

	fetchURL := info.URL
	if apikey, err := indexer.GetAPIKey(); err == nil && apikey != "" {
		if u, perr := url.Parse(fetchURL); perr == nil {
			q := u.Query()
			if q.Get("apikey") != "" {
				q.Set("apikey", apikey)
				u.RawQuery = q.Encode()
				fetchURL = u.String()
			}
		}
	}

	return nzb_info.FetchNZBFile(fetchURL, info.Name, log,
		nzb_info.WithIndexerId(indexerId),
		nzb_info.WithOnFetched(func(nzbFile *nzb_info.NZBFile, err error, latency time.Duration) {
			var bytes int64
			if nzbFile != nil {
				bytes = int64(len(nzbFile.Blob))
			}
			newznab_stats.RecordDownload(indexerId, latency, bytes, err)
		}),
	)
}
