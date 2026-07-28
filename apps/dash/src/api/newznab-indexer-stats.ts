import { useQuery } from "@tanstack/react-query";

import { api } from "@/lib/api";

const MINUTE = 60 * 1000;

export type AggregatedIndexerStats = {
  avg_download_latency_ms: number;
  avg_result_count: number;
  avg_search_latency_ms: number;
  download_bytes: number;
  download_count: number;
  download_error_count: number;
  errors_by_type: Record<string, number>;
  indexer_id: number;
  indexer_name: string;
  last_seen_at: string;
  p50_search_latency_ms: number;
  p95_search_latency_ms: number;
  p99_search_latency_ms: number;
  result_total: number;
  search_bytes: number;
  search_count: number;
  search_error: number;
  search_ok: number;
  success_rate: number;
  zero_result_count: number;
  zero_result_rate: number;
};

export type IndexerTimeSeries = {
  buckets: IndexerTimeSeriesBucket[];
  name: string;
};

export type IndexerTimeSeriesBucket = {
  avg_latency_ms: number;
  download_bytes: number;
  download_count: number;
  search_count: number;
  search_error: number;
  time: string;
};

type NewznabIndexerStatsHistoryData = {
  items: AggregatedIndexerStats[];
};

type NewznabIndexerStatsTimeSeriesData = {
  items: Record<string, IndexerTimeSeries>;
};

export function useNewznabIndexerStatsHistory(range: string) {
  return useQuery({
    queryFn: async () => {
      const { data } = await api<NewznabIndexerStatsHistoryData>(
        `/stats/newznab-indexers/history?range=${range}`,
      );
      return data;
    },
    queryKey: ["/stats/newznab-indexers/history", range],
    staleTime: 5 * MINUTE,
  });
}

export function useNewznabIndexerStatsTimeSeries(range: string) {
  return useQuery({
    queryFn: async () => {
      const { data } = await api<NewznabIndexerStatsTimeSeriesData>(
        `/stats/newznab-indexers/timeseries?range=${range}`,
      );
      return data;
    },
    queryKey: ["/stats/newznab-indexers/timeseries", range],
    staleTime: 5 * MINUTE,
  });
}
