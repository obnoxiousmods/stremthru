import prettyBytes from "pretty-bytes";

import type { ChartConfig } from "@/components/ui/chart";

export type TimeRange = "7d" | "24h" | "30d";

export const TIME_RANGES: TimeRange[] = ["24h", "7d", "30d"];

export const CHART_COLORS = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
];

export const formatBytes = (v: number) => prettyBytes(v);
export const formatBytesPerSec = (v: number) => `${prettyBytes(v)}/s`;
export const formatMs = (v: number) => `${v.toFixed(0)} ms`;
export const formatPercent = (v: number) => `${v.toFixed(1)}%`;

export function buildChartConfig<T>(
  items: T[],
  keyFn: (item: T) => string,
  labelFn: (item: T) => string,
): ChartConfig {
  const config: ChartConfig = {};
  items.forEach((item, i) => {
    config[keyFn(item)] = {
      color: CHART_COLORS[i % CHART_COLORS.length],
      label: labelFn(item),
    };
  });
  return config;
}

export function formatLastSeen(iso: string): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString();
}

export function formatTime(isoTime: string, range: string): string {
  const d = new Date(isoTime);
  if (range === "24h") {
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }
  return d.toLocaleDateString([], { day: "numeric", month: "short" });
}

export function pivotTimeSeries<TBucket extends { time: string }>(
  series: Record<string, { buckets: TBucket[]; name: string }>,
  range: string,
  extractValue: (bucket: TBucket) => number,
) {
  const ids = Object.keys(series).sort((a, b) =>
    series[a].name.localeCompare(series[b].name),
  );

  const chartConfig = buildChartConfig(
    ids,
    (id) => id,
    (id) => series[id].name,
  );

  const timeMap = new Map<string, Record<string, number>>();
  for (const [id, ts] of Object.entries(series)) {
    for (const bucket of ts.buckets) {
      if (!timeMap.has(bucket.time)) {
        timeMap.set(bucket.time, {});
      }
      timeMap.get(bucket.time)![id] = extractValue(bucket);
    }
  }

  const chartData = Array.from(timeMap.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([time, values]) => ({
      time: formatTime(time, range),
      ...values,
    }));

  return { chartConfig, chartData, ids };
}
