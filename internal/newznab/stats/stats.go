package newznab_stats

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MunifTanjim/stremthru/core"
)

type Operation string

const (
	OperationSearch   Operation = "search"
	OperationDownload Operation = "download"
)

type ErrorType string

const (
	ErrorTypeNetwork   ErrorType = "network"
	ErrorTypeHTTP4xx   ErrorType = "http_4xx"
	ErrorTypeHTTP5xx   ErrorType = "http_5xx"
	ErrorTypeTimeout   ErrorType = "timeout"
	ErrorTypeParse     ErrorType = "parse"
	ErrorTypeRateLimit ErrorType = "rate_limit"
	ErrorTypeUnknown   ErrorType = "unknown"
)

type Event struct {
	IndexerId   int64
	Operation   Operation
	ErrorType   ErrorType
	Error       string
	HTTPStatus  int
	LatencyMs   float64
	ResultCount int64
	Bytes       int64
}

const (
	eventChanBufferSize = 1024
	batchMaxSize        = 100
)

var eventCh = make(chan Event, eventChanBufferSize)
var shuttingDown atomic.Bool

func Record(e Event) {
	if shuttingDown.Load() {
		return
	}
	if e.IndexerId == 0 {
		return
	}
	select {
	case eventCh <- e:
	default:
		log.Warn("newznab stats event channel full, dropping event", "indexer_id", e.IndexerId, "operation", e.Operation)
	}
}

func RecordSearch(indexerId int64, latency time.Duration, resultCount int, bytes int64, err error) {
	e := Event{
		IndexerId:   indexerId,
		Operation:   OperationSearch,
		LatencyMs:   float64(latency.Microseconds()) / 1000.0,
		ResultCount: int64(resultCount),
		Bytes:       bytes,
	}
	applyError(&e, err)
	Record(e)
}

func RecordDownload(indexerId int64, latency time.Duration, bytes int64, err error) {
	e := Event{
		IndexerId: indexerId,
		Operation: OperationDownload,
		LatencyMs: float64(latency.Microseconds()) / 1000.0,
		Bytes:     bytes,
	}
	applyError(&e, err)
	Record(e)
}

func applyError(e *Event, err error) {
	if err == nil {
		return
	}
	e.ErrorType = classifyError(err)
	e.HTTPStatus = extractStatusCode(err)
	e.Error = sanitizeErrorMessage(err.Error())
}

const maxErrorMessageLen = 500

var apiKeyParamRegex = regexp.MustCompile(`(?i)([?&;])apikey=[^&;\s]*`)

func sanitizeErrorMessage(msg string) string {
	msg = apiKeyParamRegex.ReplaceAllString(msg, "${1}apikey=REDACTED")
	if len(msg) > maxErrorMessageLen {
		msg = msg[:maxErrorMessageLen]
	}
	return msg
}

func RecordRateLimited(indexerId int64, operation Operation) {
	Record(Event{
		IndexerId: indexerId,
		Operation: operation,
		ErrorType: ErrorTypeRateLimit,
	})
}

func classifyError(err error) ErrorType {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorTypeTimeout
	}
	if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
		return ErrorTypeTimeout
	}
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		if urlErr.Timeout() {
			return ErrorTypeTimeout
		}
		return ErrorTypeNetwork
	}
	if upErr, ok := errors.AsType[*core.UpstreamError](err); ok {
		switch {
		case upErr.StatusCode >= 500:
			return ErrorTypeHTTP5xx
		case upErr.StatusCode >= 400:
			if upErr.StatusCode == 429 {
				return ErrorTypeRateLimit
			}
			return ErrorTypeHTTP4xx
		}
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "rate limit"):
		return ErrorTypeRateLimit
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return ErrorTypeTimeout
	case strings.Contains(msg, "unmarshal"), strings.Contains(msg, "parse"), strings.Contains(msg, "xml"):
		return ErrorTypeParse
	case strings.Contains(msg, "connection"), strings.Contains(msg, "dial"), strings.Contains(msg, "no such host"):
		return ErrorTypeNetwork
	}
	return ErrorTypeUnknown
}

func extractStatusCode(err error) int {
	if err == nil {
		return 0
	}
	if upErr, ok := errors.AsType[*core.UpstreamError](err); ok {
		return upErr.StatusCode
	}
	return 0
}

var (
	writerStartOnce sync.Once
	writerDone      = make(chan struct{})
)

func startWriter() {
	writerStartOnce.Do(func() {
		go writerLoop()
	})
}

func writerLoop() {
	defer close(writerDone)
	for {
		select {
		case <-backgroundJobQuit:
			// Drain remaining events before exiting.
			for {
				select {
				case e := <-eventCh:
					flushBatch(e)
				default:
					return
				}
			}
		case e := <-eventCh:
			flushBatch(e)
		}
	}
}

// flushBatch starts a batch with the given event, opportunistically drains any
// other events already in the channel (up to batchMaxSize), then issues one
// InsertBatch. During quiet periods this is 1 event per INSERT; during bursts
// it coalesces.
func flushBatch(first Event) {
	buf := make([]NewznabIndexerStat, 0, batchMaxSize)
	buf = append(buf, eventToStat(first))
	for len(buf) < batchMaxSize {
		select {
		case e := <-eventCh:
			buf = append(buf, eventToStat(e))
		default:
			if err := InsertBatch(buf); err != nil {
				log.Error("failed to insert newznab indexer stats batch", "error", err, "count", len(buf))
			}
			return
		}
	}
	if err := InsertBatch(buf); err != nil {
		log.Error("failed to insert newznab indexer stats batch", "error", err, "count", len(buf))
	}
}

func eventToStat(e Event) NewznabIndexerStat {
	stat := NewznabIndexerStat{
		IndexerId:   e.IndexerId,
		Operation:   string(e.Operation),
		ErrorType:   string(e.ErrorType),
		HTTPStatus:  e.HTTPStatus,
		LatencyMs:   e.LatencyMs,
		ResultCount: e.ResultCount,
		Bytes:       e.Bytes,
	}
	if e.Error != "" {
		stat.Error = sql.NullString{String: e.Error, Valid: true}
	}
	return stat
}
