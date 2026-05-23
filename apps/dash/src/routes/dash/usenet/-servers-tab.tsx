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
  AggregatedServerStats,
  ServerTimeSeries,
  TimeSeriesBucket,
} from "@/api/usenet-stats";

import {
  useUsenetServerStatsHistory,
  useUsenetServerStatsTimeSeries,
} from "@/api/usenet-stats";
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

import { RangeSelector } from "./-range-selector";
import {
  CHART_COLORS,
  formatBytes,
  formatBytesPerSec,
  formatMs,
  formatPercent,
  pivotTimeSeries,
  type TimeRange,
} from "./-shared";
import { SummaryCard } from "./-summary-card";

const EMPTY_SERVERS_MAP: Record<string, ServerTimeSeries> = {};
const EMPTY_SERVERS_LIST: AggregatedServerStats[] = [];

const LATENCY_CHART_CONFIG = {
  avg_latency_ms: { color: "var(--chart-1)", label: "Avg." },
  p50_latency_ms: { color: "var(--chart-2)", label: "p50" },
  p95_latency_ms: { color: "var(--chart-3)", label: "p95" },
  p99_latency_ms: { color: "var(--chart-4)", label: "p99" },
} satisfies ChartConfig;

const SEGMENTS_BAR_CHART_CONFIG = {
  article_not_found: { color: "var(--chart-3)", label: "Not Found" },
  connection_errors: { color: "var(--chart-5)", label: "Conn Errors" },
  segments_fetched: { color: "var(--chart-1)", label: "Segments" },
} satisfies ChartConfig;

function buildServerChartConfig(servers: AggregatedServerStats[]): ChartConfig {
  const config: ChartConfig = {};
  servers.forEach((s, i) => {
    config[s.server_id] = {
      color: CHART_COLORS[i % CHART_COLORS.length],
      label: s.server_name,
    };
  });
  return config;
}

const extractDownloadedData = (s: AggregatedServerStats) => s.bytes_downloaded;
const extractErrors = (s: AggregatedServerStats) =>
  s.article_not_found + s.connection_errors;
const extractSpeed = (s: AggregatedServerStats) => s.throughput_bps;
const extractNZBCount = (s: AggregatedServerStats) => s.nzb_count;

function ChartsView({
  range,
  servers,
}: {
  range: string;
  servers: AggregatedServerStats[];
}) {
  return (
    <>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <ServerPieChart
          extractValue={extractDownloadedData}
          servers={servers}
          title="Downloaded Data"
          tooltipValueFormatter={formatBytes}
        />
        <ServerPieChart
          emptyMessage="No errors recorded"
          extractValue={extractErrors}
          servers={servers}
          title="Error Distribution"
        />
        <ServerHorizontalBarChart
          extractValue={extractSpeed}
          formatValue={formatBytesPerSec}
          servers={servers}
          title="Download Speed"
        />
        <ServerHorizontalBarChart
          extractValue={extractNZBCount}
          servers={servers}
          title="NZB Count"
        />
      </div>
      <ServerComparisonBarChart servers={servers} />
      <TimeSeriesLineChart
        extractValue={extractBytesDownloaded}
        formatValue={formatBytes}
        range={range}
        title="Downloaded Data Over Time"
      />
      <TimeSeriesLineChart
        extractValue={extractThroughputBps}
        formatValue={formatBytesPerSec}
        range={range}
        title="Download Speed Over Time"
      />
      <TimeSeriesLineChart
        extractValue={extractErrorRate}
        formatValue={formatPercent}
        range={range}
        title="Error Rate Over Time"
      />
      <ServerLatencyChart servers={servers} />
      <TimeSeriesLineChart
        extractValue={extractAvgLatencyMs}
        formatValue={formatMs}
        range={range}
        title="Avg. Latency Over Time"
      />
    </>
  );
}

function ServerPieChart({
  emptyMessage = "No data",
  extractValue,
  servers,
  title,
  tooltipValueFormatter,
}: {
  emptyMessage?: string;
  extractValue: (s: AggregatedServerStats) => number;
  servers: AggregatedServerStats[];
  title: string;
  tooltipValueFormatter?: (value: number) => string;
}) {
  const chartConfig = useMemo(() => buildServerChartConfig(servers), [servers]);
  const data = useMemo(
    () =>
      servers.map((s, i) => ({
        fill: CHART_COLORS[i % CHART_COLORS.length],
        server: s.server_id,
        value: extractValue(s),
      })),
    [servers, extractValue],
  );

  const hasData = data.some((d) => d.value > 0);
  if (!hasData) {
    return (
      <Card className="py-4">
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
    <Card className="py-4">
      <CardHeader className="items-center py-0">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="pt-4">
        <ChartContainer
          className="mx-auto aspect-square max-h-[250px]"
          config={chartConfig}
        >
          <PieChart>
            <ChartTooltip
              content={
                <ChartTooltipContent valueFormatter={tooltipValueFormatter} />
              }
            />
            <Pie
              data={data}
              dataKey="value"
              innerRadius={40}
              isAnimationActive
              nameKey="server"
              outerRadius={80}
            />
            <ChartLegend content={<ChartLegendContent nameKey="server" />} />
          </PieChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}

function useTimeSeriesChartData(
  range: string,
  extractValue: (bucket: TimeSeriesBucket) => number,
) {
  const { data, isLoading } = useUsenetServerStatsTimeSeries(range);
  const servers = data?.items ?? EMPTY_SERVERS_MAP;

  const pivoted = useMemo(
    () => pivotTimeSeries(servers, range, extractValue),
    [servers, range, extractValue],
  );

  return {
    chartConfig: pivoted.chartConfig,
    chartData: pivoted.chartData,
    isLoading,
    serverIds: pivoted.ids,
  };
}

const extractBytesDownloaded = (bucket: TimeSeriesBucket) =>
  bucket.bytes_downloaded;

const extractThroughputBps = (bucket: TimeSeriesBucket) =>
  bucket.throughput_bps;

const extractAvgLatencyMs = (bucket: TimeSeriesBucket) => bucket.avg_latency_ms;

const extractErrorRate = (bucket: TimeSeriesBucket) => {
  const totalOps =
    bucket.segments_fetched +
    bucket.article_not_found +
    bucket.connection_errors;
  return totalOps > 0
    ? Math.round(
        ((bucket.article_not_found + bucket.connection_errors) / totalOps) *
          10000,
      ) / 100
    : 0;
};

export function ServersTab() {
  const [range, setRange] = useState<TimeRange>("24h");
  return (
    <div className="flex flex-col gap-4">
      <Card className="py-4">
        <CardHeader className="py-0">
          <CardTitle>Usenet Server Statistics</CardTitle>
          <CardDescription>Last {range}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-4 pt-4">
          <RangeSelector range={range} setRange={setRange} />
        </CardContent>
      </Card>

      <HistoryView range={range} />
    </div>
  );
}

function HistoryView({ range }: { range: string }) {
  const { data, isLoading } = useUsenetServerStatsHistory(range);

  const servers = data?.items ?? EMPTY_SERVERS_LIST;

  const { avgThroughput, errorRate, totalAnf, totalBytes, totalSegments } =
    useMemo(() => {
      let totalSegments = 0;
      let totalBytes = 0;
      let totalAnf = 0;
      let totalConnErr = 0;
      let totalFetchTime = 0;
      for (const v of servers) {
        totalSegments += v.segments_fetched;
        totalBytes += v.bytes_downloaded;
        totalAnf += v.article_not_found;
        totalConnErr += v.connection_errors;
        if (v.throughput_bps > 0) {
          totalFetchTime += v.bytes_downloaded / v.throughput_bps;
        }
      }
      const totalOps = totalSegments + totalAnf + totalConnErr;
      const errorRate =
        totalOps > 0
          ? (((totalAnf + totalConnErr) / totalOps) * 100).toFixed(2)
          : "0";
      const avgThroughput =
        totalFetchTime > 0 ? totalBytes / totalFetchTime : 0;
      return { avgThroughput, errorRate, totalAnf, totalBytes, totalSegments };
    }, [servers]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Spinner />
      </div>
    );
  }

  if (servers.length === 0) {
    return (
      <p className="text-muted-foreground py-8 text-center text-sm">
        No historical data for this time range
      </p>
    );
  }

  return (
    <>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
        <SummaryCard label="Segments Fetched" value={String(totalSegments)} />
        <SummaryCard label="Downloaded" value={prettyBytes(totalBytes)} />
        <SummaryCard label="Article Not Found" value={String(totalAnf)} />
        <SummaryCard label="Error Rate" value={`${errorRate}%`} />
        <SummaryCard
          label="Avg Speed"
          value={avgThroughput > 0 ? `${prettyBytes(avgThroughput)}/s` : "N/A"}
        />
      </div>

      <ChartsView range={range} servers={servers} />
    </>
  );
}

function ServerComparisonBarChart({
  servers,
}: {
  servers: AggregatedServerStats[];
}) {
  const data = useMemo(
    () =>
      servers.map((s) => ({
        article_not_found: s.article_not_found,
        connection_errors: s.connection_errors,
        segments_fetched: s.segments_fetched,
        server: s.server_name,
      })),
    [servers],
  );

  return (
    <Card className="py-4">
      <CardHeader className="py-0">
        <CardTitle className="text-sm">Server Comparison</CardTitle>
      </CardHeader>
      <CardContent className="pt-4">
        <ChartContainer
          className="max-h-[300px] w-full"
          config={SEGMENTS_BAR_CHART_CONFIG}
        >
          <BarChart data={data}>
            <CartesianGrid vertical={false} />
            <XAxis axisLine={false} dataKey="server" tickLine={false} />
            <YAxis axisLine={false} tickLine={false} />
            <ChartTooltip content={<ChartTooltipContent />} />
            <ChartLegend content={<ChartLegendContent />} />
            <Bar
              dataKey="segments_fetched"
              fill="var(--color-segments_fetched)"
              radius={[4, 4, 0, 0]}
            />
            <Bar
              dataKey="article_not_found"
              fill="var(--color-article_not_found)"
              radius={[4, 4, 0, 0]}
            />
            <Bar
              dataKey="connection_errors"
              fill="var(--color-connection_errors)"
              radius={[4, 4, 0, 0]}
            />
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}

function ServerHorizontalBarChart({
  extractValue,
  formatValue,
  servers,
  title,
}: {
  extractValue: (s: AggregatedServerStats) => number;
  formatValue?: (value: number) => string;
  servers: AggregatedServerStats[];
  title: string;
}) {
  const chartConfig = useMemo(() => buildServerChartConfig(servers), [servers]);
  const data = useMemo(
    () =>
      servers.map((s, i) => ({
        fill: CHART_COLORS[i % CHART_COLORS.length],
        server: s.server_name,
        value: extractValue(s),
      })),
    [servers, extractValue],
  );

  const hasData = data.some((d) => d.value > 0);
  if (!hasData) {
    return (
      <Card className="py-4">
        <CardHeader className="items-center py-0">
          <CardTitle className="text-sm">{title}</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-center pt-4">
          <p className="text-muted-foreground text-sm">No data</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="py-4">
      <CardHeader className="items-center py-0">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="pt-4">
        <ChartContainer className="max-h-[250px] w-full" config={chartConfig}>
          <BarChart data={data} layout="vertical">
            <CartesianGrid horizontal={false} />
            <XAxis
              axisLine={false}
              tickFormatter={formatValue}
              tickLine={false}
              type="number"
            />
            <YAxis
              axisLine={false}
              dataKey="server"
              tickLine={false}
              type="category"
              width={100}
            />
            <ChartTooltip
              content={<ChartTooltipContent valueFormatter={formatValue} />}
            />
            <Bar dataKey="value" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}

function ServerLatencyChart({ servers }: { servers: AggregatedServerStats[] }) {
  const data = useMemo(
    () =>
      servers.map((s) => ({
        avg_latency_ms: Math.round(s.avg_latency_ms * 100) / 100,
        p50_latency_ms: Math.round(s.p50_latency_ms * 100) / 100,
        p95_latency_ms: Math.round(s.p95_latency_ms * 100) / 100,
        p99_latency_ms: Math.round(s.p99_latency_ms * 100) / 100,
        server: s.server_name,
      })),
    [servers],
  );

  const hasData = data.some((d) => d.avg_latency_ms > 0);
  if (!hasData) {
    return (
      <Card className="py-4">
        <CardHeader className="py-0">
          <CardTitle className="text-sm">Latency</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-center pt-4">
          <p className="text-muted-foreground text-sm">No data</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="py-4">
      <CardHeader className="py-0">
        <CardTitle className="text-sm">Latency</CardTitle>
      </CardHeader>
      <CardContent className="pt-4">
        <ChartContainer
          className="max-h-[300px] w-full"
          config={LATENCY_CHART_CONFIG}
        >
          <LineChart data={data}>
            <CartesianGrid vertical={false} />
            <XAxis axisLine={false} dataKey="server" tickLine={false} />
            <YAxis axisLine={false} tickFormatter={formatMs} tickLine={false} />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  indicator="line"
                  valueFormatter={(value) => formatMs(value as number)}
                />
              }
            />
            <ChartLegend content={<ChartLegendContent />} />
            <Line
              dataKey="avg_latency_ms"
              dot
              stroke="var(--color-avg_latency_ms)"
              strokeWidth={2}
              type="monotone"
            />
            <Line
              dataKey="p50_latency_ms"
              dot
              stroke="var(--color-p50_latency_ms)"
              strokeWidth={2}
              type="monotone"
            />
            <Line
              dataKey="p95_latency_ms"
              dot
              stroke="var(--color-p95_latency_ms)"
              strokeWidth={2}
              type="monotone"
            />
            <Line
              dataKey="p99_latency_ms"
              dot
              stroke="var(--color-p99_latency_ms)"
              strokeWidth={2}
              type="monotone"
            />
          </LineChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}

function TimeSeriesLineChart({
  extractValue,
  formatValue,
  range,
  title,
}: {
  extractValue: (bucket: TimeSeriesBucket) => number;
  formatValue: (value: number) => string;
  range: string;
  title: string;
}) {
  const { chartConfig, chartData, isLoading, serverIds } =
    useTimeSeriesChartData(range, extractValue);

  if (isLoading) {
    return (
      <Card className="py-4">
        <CardHeader className="py-0">
          <CardTitle className="text-sm">{title}</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-center py-12">
          <Spinner />
        </CardContent>
      </Card>
    );
  }

  if (serverIds.length === 0) return null;

  return (
    <Card className="py-4">
      <CardHeader className="py-0">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="pt-4">
        <ChartContainer className="max-h-[300px] w-full" config={chartConfig}>
          <LineChart data={chartData}>
            <CartesianGrid vertical={false} />
            <XAxis axisLine={false} dataKey="time" tickLine={false} />
            <YAxis
              axisLine={false}
              tickFormatter={formatValue}
              tickLine={false}
            />
            <ChartTooltip
              content={<ChartTooltipContent valueFormatter={formatValue} />}
            />
            <ChartLegend content={<ChartLegendContent />} />
            {serverIds.map((id) => (
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
