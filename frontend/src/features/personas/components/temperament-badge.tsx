import { Badge } from "@/components/ui/badge";
import type { Temperament } from "@/lib/api/types";

import { TEMPERAMENT_LABELS } from "../lib/form";

export function TemperamentBadge({ temperament }: { temperament: Temperament }) {
  return (
    <Badge variant="secondary" className="font-normal">
      {TEMPERAMENT_LABELS[temperament]}
    </Badge>
  );
}
