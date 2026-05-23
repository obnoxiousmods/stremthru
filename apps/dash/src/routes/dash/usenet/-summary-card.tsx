import { Card, CardContent } from "@/components/ui/card";

export function SummaryCard({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <Card className="grow py-4">
      <CardContent className="flex flex-col items-center gap-1 px-4 py-0">
        <span className="text-muted-foreground text-xs">{label}</span>
        <span className="text-lg font-semibold">{value}</span>
      </CardContent>
    </Card>
  );
}
