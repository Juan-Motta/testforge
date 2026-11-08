import { AudioLinesIcon, MessageSquareTextIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import type { Modality } from "@/lib/api/types";

import { MODALITY_LABELS } from "../lib/display";

export function ModalityBadge({ modality }: { modality: Modality }) {
  const Icon = modality === "voice" ? AudioLinesIcon : MessageSquareTextIcon;
  return (
    <Badge variant="secondary" className="gap-1 font-normal">
      <Icon aria-hidden />
      {MODALITY_LABELS[modality]}
    </Badge>
  );
}
