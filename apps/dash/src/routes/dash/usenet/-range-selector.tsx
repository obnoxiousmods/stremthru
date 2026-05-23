import { Button } from "@/components/ui/button";
import { ButtonGroup } from "@/components/ui/button-group";

import { TIME_RANGES, type TimeRange } from "./-shared";

export function RangeSelector({
  range,
  setRange,
}: {
  range: TimeRange;
  setRange: (r: TimeRange) => void;
}) {
  return (
    <ButtonGroup>
      {TIME_RANGES.map((r) => (
        <Button
          key={r}
          onClick={() => setRange(r)}
          size="sm"
          variant={range === r ? "default" : "outline"}
        >
          {r}
        </Button>
      ))}
    </ButtonGroup>
  );
}
