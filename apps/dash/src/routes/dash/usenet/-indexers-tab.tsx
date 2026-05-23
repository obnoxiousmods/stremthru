import prettyBytes from "pretty-bytes";
import { useMemo, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  Pie,
  PieChart,
  XAxis,
  YAxis,
} from "recharts";

import type {
  AggregatedIndexerStats,
  IndexerTimeSeries,
} from "@/api/newznab-indexer-stats";

import {
  useNewznabIndexerStatsHistory,
  useNewznabIndexerStatsTimeSeries,
} from "@/api/newznab-indexer-stats";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  type ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";
import { Spinner } from "@/components/ui/spinner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";

import { RangeSelector } from "./-range-selector";
import {
  buildChartConfig,
  CHART_COLORS,
  formatBytes,
  formatLastSeen,
  formatMs,
  formatPercent,
  pivotTimeSeries,
  type TimeRange,
} from "./-shared";
import { SummaryCard } from "./-summary-card";

const EMPTY_INDEXERS_LIST: AggregatedIndexerStats[] = [];
const EMPTY_TS_MAP: Record<string, IndexerTimeSeries> = {};

const ERROR_TYPES = [
  "network",
  "http_4xx",
  "http_5xx",
  "timeout",
  "parse",
  "rate_limit",
  "unknown",
] as const;

const ERROR_CHART_CONFIG = {
  http_4xx: { color: "var(--chart-2)", label: "HTTP 4xx" },
  http_5xx: { color: "var(--chart-3)", label: "HTTP 5xx" },
  network: { color: "var(--chart-1)", label: "Network" },
  parse: { color: "var(--chart-5)", label: "Parse" },
  rate_limit: { color: "var(--chart-1)", label: "Rate Limit" },
  timeout: { color: "var(--chart-4)", label: "Timeout" },
  unknown: { color: "var(--chart-2)", label: "Unknown" },
} satisfies ChartConfig;

const LATENCY_CHART_CONFIG = {
  avg: { color: "var(--chart-1)", label: "Avg" },
  p95: { color: "var(--chart-3)", label: "p95" },
} satisfies ChartConfig;

type IndexerColumn = {
  align?: "right";
  key: SortKey;
  label: string;
  render: (idx: AggregatedIndexerStats) => React.ReactNode;
};

type SortKey = keyof Pick<
  AggregatedIndexerStats,
  | "avg_search_latency_ms"
  | "download_bytes"
  | "download_count"
  | "indexer_name"
  | "last_seen_at"
  | "p95_search_latency_ms"
  | "search_bytes"
  | "search_count"
  | "success_rate"
  | "zero_result_rate"
>;

const INDEXER_COLUMNS: IndexerColumn[] = [
  { key: "indexer_name", label: "Name", render: (i) => i.indexer_name },
  {
    align: "right",
    key: "search_count",
    label: "Searches",
    render: (i) => i.search_count,
  },
  {
    align: "right",
    key: "success_rate",
    label: "Success %",
    render: (i) => formatPercent(i.success_rate),
  },
  {
    align: "right",
    key: "avg_search_latency_ms",
    label: "Avg Latency",
    render: (i) => formatMs(i.avg_search_latency_ms),
  },
  {
    align: "right",
    key: "p95_search_latency_ms",
    label: "p95 Latency",
    render: (i) => formatMs(i.p95_search_latency_ms),
  },
  {
    align: "right",
    key: "zero_result_rate",
    label: "Zero-Result %",
    render: (i) => formatPercent(i.zero_result_rate),
  },
  {
    align: "right",
    key: "search_bytes",
    label: "Search Bandwidth",
    render: (i) => prettyBytes(i.search_bytes),
  },
  {
    align: "right",
    key: "download_count",
    label: "Downloads",
    render: (i) => i.download_count,
  },
  {
    align: "right",
    key: "download_bytes",
    label: "Download Bandwidth",
    render: (i) => prettyBytes(i.download_bytes),
  },
  {
    key: "last_seen_at",
    label: "Last Seen",
    render: (i) => formatLastSeen(i.last_seen_at),
  },
];

export function IndexersTab() {
  const [range, setRange] = useState<TimeRange>("24h");
  return (
    <div className="flex flex-col gap-4">
      <Card className="py-4">
        <CardHeader className="py-0">
          <CardTitle>Newznab Indexer Statistics</CardTitle>
          <CardDescription>Last {range}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-4">
          <RangeSelector range={range} setRange={setRange} />
        </CardContent>
      </Card>

      <HistoryView range={range} />
    </div>
  );
}

function IndexerPieChart({
  emptyMessage = "No data",
  extractValue,
  indexers,
  title,
  tooltipValueFormatter,
}: {
  emptyMessage?: string;
  extractValue: (i: AggregatedIndexerStats) => number;
  indexers: AggregatedIndexerStats[];
  title: string;
  tooltipValueFormatter?: (value: number) => string;
}) {
  const chartConfig = useMemo(
    () =>
      buildChartConfig(
        indexers,
        (i) => String(i.indexer_id),
        (i) => i.indexer_name,
      ),
    [indexers],
  );
  const data = useMemo(
    () =>
      indexers.map((i, idx) => ({
        fill: CHART_COLORS[idx % CHART_COLORS.length],
        indexer: String(i.indexer_id),
        indexer_name: i.indexer_name,
        value: extractValue(i),
      })),
    [indexers, extractValue],
  );

  const hasData = data.some((d) => d.value > 0);
  if (!hasData) {
    return (
      <Card className="grow basis-1 py-4">
        <CardHeader className="items-center py-0">
          <CardTitle className="text-sm">{title}</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-center pt-4">
          <p className="text-muted-foreground text-sm">{emptyMessage}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="min-w-96 grow basis-1 py-4">
      <CardHeader className="items-center py-0">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <ChartContainer
          className="mx-auto aspect-square max-h-[250px] w-full overflow-visible"
          config={chartConfig}
        >
          <PieChart>
            <ChartTooltip
              content={
                <ChartTooltipContent
                  nameKey="indexer"
                  valueFormatter={tooltipValueFormatter}
                />
              }
            />
            <Pie
              data={data}
              dataKey="value"
              innerRadius={40}
              isAnimationActive
              label={({ payload, ...props }) => {
                return (
                  <text
                    cx={props.cx}
                    cy={props.cy}
                    dominantBaseline={props.dominantBaseline}
                    fill="var(--foreground)"
                    textAnchor={props.textAnchor}
                    x={props.x}
                    y={props.y}
                  >
                    {payload.indexer_name}
                  </text>
                );
              }}
              nameKey="indexer"
              outerRadius={80}
            />
          </PieChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}

const extractResultTotal = (i: AggregatedIndexerStats) => i.result_total;
const extractSearchBytes = (i: AggregatedIndexerStats) => i.search_bytes;
const extractDownloadBytes = (i: AggregatedIndexerStats) => i.download_bytes;

function ErrorBreakdownChart({
  indexers,
}: {
  indexers: AggregatedIndexerStats[];
}) {
  const data = useMemo(
    () =>
      indexers
        .map((idx) => {
          const row: Record<string, number | string> = {
            indexer: idx.indexer_name,
          };
          for (const t of ERROR_TYPES) {
            row[t] = idx.errors_by_type?.[t] ?? 0;
          }
          return row;
        })
        .filter((r) => ERROR_TYPES.some((t) => (r[t] as number) > 0)),
    [indexers],
  );

  if (data.length === 0) {
    return (
      <Card className="py-4">
        <CardHeader className="py-0">
          <CardTitle className="text-sm">Error Breakdown</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-center py-4">
          <p className="text-muted-foreground text-sm">No errors recorded</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="py-4">
      <CardHeader className="py-0">
        <CardTitle className="text-sm">Error Breakdown</CardTitle>
      </CardHeader>
      <CardContent className="pt-4">
        <ChartContainer
          className="max-h-[300px] w-full"
          config={ERROR_CHART_CONFIG}
        >
          <BarChart data={data} layout="vertical">
            <CartesianGrid horizontal={false} />
            <XAxis axisLine={false} tickLine={false} type="number" />
            <YAxis
              axisLine={false}
              dataKey="indexer"
              tickLine={false}
              type="category"
              width={120}
            />
            <ChartTooltip content={<ChartTooltipContent />} />
            <ChartLegend content={<ChartLegendContent />} />
            {ERROR_TYPES.map((t) => (
              <Bar
                dataKey={t}
                fill={`var(--color-${t})`}
                key={t}
                stackId="errors"
              />
            ))}
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}

function HistoryView({ range }: { range: string }) {
  const { data, isLoading } = useNewznabIndexerStatsHistory(range);
  const indexers = data?.items ?? EMPTY_INDEXERS_LIST;

  const summary = useMemo(() => {
    let totalSearches = 0;
    let totalSearchOk = 0;
    let totalSearchBytes = 0;
    let totalDownloads = 0;
    let totalDownloadBytes = 0;
    let latencyWeighted = 0;
    let latencyCount = 0;
    for (const v of indexers) {
      totalSearches += v.search_count;
      totalSearchOk += v.search_ok;
      totalSearchBytes += v.search_bytes;
      totalDownloads += v.download_count;
      totalDownloadBytes += v.download_bytes;
      if (v.search_ok > 0 && v.avg_search_latency_ms > 0) {
        latencyWeighted += v.avg_search_latency_ms * v.search_ok;
        latencyCount += v.search_ok;
      }
    }
    const errorRate =
      totalSearches > 0
        ? ((totalSearches - totalSearchOk) / totalSearches) * 100
        : 0;
    const avgLatency = latencyCount > 0 ? latencyWeighted / latencyCount : 0;
    return {
      avgLatency,
      errorRate,
      totalDownloadBytes,
      totalDownloads,
      totalSearchBytes,
      totalSearches,
    };
  }, [indexers]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Spinner />
      </div>
    );
  }

  if (indexers.length === 0) {
    return (
      <p className="text-muted-foreground py-8 text-center text-sm">
        No indexer data for this time range
      </p>
    );
  }

  return (
    <>
      <div className="flex flex-row flex-wrap justify-around gap-4">
        <SummaryCard
          label="Total Searches"
          value={String(summary.totalSearches)}
        />
        <SummaryCard
          label="Search Error Rate"
          value={formatPercent(summary.errorRate)}
        />
        <SummaryCard
          label="Avg Search Latency"
          value={summary.avgLatency > 0 ? formatMs(summary.avgLatency) : "N/A"}
        />
        <SummaryCard
          label="Search Bandwidth"
          value={prettyBytes(summary.totalSearchBytes)}
        />
        <SummaryCard
          label="Search Bandwidth"
          value={prettyBytes(summary.totalSearchBytes)}
        />
        <SummaryCard
          label="Total Downloads"
          value={String(summary.totalDownloads)}
        />
        <SummaryCard
          label="Download Bandwidth"
          value={prettyBytes(summary.totalDownloadBytes)}
        />
      </div>

      <IndexerTable indexers={indexers} />

      <div className="flex flex-row flex-wrap gap-4">
        <IndexerPieChart
          emptyMessage="No results returned"
          extractValue={extractResultTotal}
          indexers={indexers}
          title="Result Distribution"
        />
        <IndexerPieChart
          emptyMessage="No search bandwidth recorded"
          extractValue={extractSearchBytes}
          indexers={indexers}
          title="Search Bandwidth"
          tooltipValueFormatter={(v) => formatBytes(v as number)}
        />
        <IndexerPieChart
          emptyMessage="No downloads recorded"
          extractValue={extractDownloadBytes}
          indexers={indexers}
          title="Download Bandwidth"
          tooltipValueFormatter={(v) => formatBytes(v as number)}
        />
      </div>
      <ErrorBreakdownChart indexers={indexers} />
      <LatencyComparisonChart indexers={indexers} />
      <SearchVolumeTimeSeries range={range} />
    </>
  );
}

function IndexerTable({ indexers }: { indexers: AggregatedIndexerStats[] }) {
  const [sort, setSort] = useState<{ dir: "asc" | "desc"; key: SortKey }>({
    dir: "desc",
    key: "search_count",
  });

  const sorted = useMemo(() => {
    const copy = [...indexers];
    copy.sort((a, b) => {
      const av = a[sort.key];
      const bv = b[sort.key];
      const cmp =
        typeof av === "string" && typeof bv === "string"
          ? av.localeCompare(bv)
          : (av as number) - (bv as number);
      return sort.dir === "asc" ? cmp : -cmp;
    });
    return copy;
  }, [indexers, sort]);

  const onSort = (key: SortKey) => {
    setSort((prev) =>
      prev.key === key
        ? { dir: prev.dir === "asc" ? "desc" : "asc", key }
        : { dir: "desc", key },
    );
  };

  const arrow = (key: SortKey) =>
    sort.key === key ? (sort.dir === "asc" ? " ↑" : " ↓") : "";

  return (
    <Card className="py-4">
      <CardHeader className="py-0">
        <CardTitle className="text-sm">Indexer Comparison</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              {INDEXER_COLUMNS.map((c) => (
                <TableHead
                  className={`cursor-pointer ${c.align === "right" ? "text-right" : ""}`}
                  key={c.key}
                  onClick={() => onSort(c.key)}
                >
                  {c.label}
                  {arrow(c.key)}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {sorted.map((idx) => (
              <TableRow key={idx.indexer_id}>
                {INDEXER_COLUMNS.map((c) => (
                  <TableCell
                    className={cn(
                      c.key === "indexer_name" && "font-medium",
                      c.align === "right" && "text-right",
                    )}
                    key={c.key}
                  >
                    {c.render(idx)}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

function LatencyComparisonChart({
  indexers,
}: {
  indexers: AggregatedIndexerStats[];
}) {
  const data = useMemo(
    () =>
      indexers.map((idx) => ({
        avg: idx.avg_search_latency_ms,
        indexer: idx.indexer_name,
        p95: idx.p95_search_latency_ms,
      })),
    [indexers],
  );

  const hasData = data.some((d) => d.avg > 0 || d.p95 > 0);
  if (!hasData) {
    return (
      <Card className="py-4">
        <CardHeader className="py-0">
          <CardTitle className="text-sm">Search Latency</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-center">
          <p className="text-muted-foreground text-sm">No data</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="py-4">
      <CardHeader className="py-0">
        <CardTitle className="text-sm">Search Latency</CardTitle>
      </CardHeader>
      <CardContent>
        <ChartContainer
          className="max-h-[300px] w-full"
          config={LATENCY_CHART_CONFIG}
        >
          <BarChart data={data}>
            <CartesianGrid vertical={false} />
            <XAxis axisLine={false} dataKey="indexer" tickLine={false} />
            <YAxis axisLine={false} tickFormatter={formatMs} tickLine={false} />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  valueFormatter={(v) => formatMs(v as number)}
                />
              }
            />
            <ChartLegend content={<ChartLegendContent />} />
            <Bar dataKey="avg" fill="var(--color-avg)" radius={[4, 4, 0, 0]} />
            <Bar dataKey="p95" fill="var(--color-p95)" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}

function SearchVolumeTimeSeries({ range }: { range: string }) {
  const { data, isLoading } = useNewznabIndexerStatsTimeSeries(range);
  const indexers = data?.items ?? EMPTY_TS_MAP;

  const {
    chartConfig,
    chartData,
    ids: indexerIds,
  } = useMemo(
    () => pivotTimeSeries(indexers, range, (b) => b.search_count),
    [indexers, range],
  );

  if (isLoading) {
    return (
      <Card className="py-4">
        <CardHeader className="py-0">
          <CardTitle className="text-sm">Search Volume Over Time</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-center py-12">
          <Spinner />
        </CardContent>
      </Card>
    );
  }

  if (indexerIds.length === 0) return null;

  return (
    <Card className="py-4">
      <CardHeader className="py-0">
        <CardTitle className="text-sm">Search Volume Over Time</CardTitle>
      </CardHeader>
      <CardContent>
        <ChartContainer className="max-h-[300px] w-full" config={chartConfig}>
          <LineChart data={chartData}>
            <CartesianGrid vertical={false} />
            <XAxis axisLine={false} dataKey="time" tickLine={false} />
            <YAxis axisLine={false} tickLine={false} />
            <ChartTooltip content={<ChartTooltipContent />} />
            <ChartLegend content={<ChartLegendContent />} />
            {indexerIds.map((id) => (
              <Line
                dataKey={id}
                dot={false}
                key={id}
                stroke={`var(--color-${id})`}
                strokeWidth={2}
                type="monotone"
              />
            ))}
          </LineChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
