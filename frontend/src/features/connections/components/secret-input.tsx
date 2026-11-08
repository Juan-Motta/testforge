"use client";

import { EyeIcon, EyeOffIcon } from "lucide-react";
import { useState, type ComponentProps } from "react";

import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";

export function SecretInput(props: ComponentProps<typeof InputGroupInput>) {
  const [visible, setVisible] = useState(false);
  return (
    <InputGroup>
      <InputGroupInput
        type={visible ? "text" : "password"}
        autoComplete="off"
        spellCheck={false}
        className="font-mono"
        {...props}
      />
      <InputGroupAddon align="inline-end">
        <InputGroupButton
          size="icon-xs"
          aria-label={visible ? "Hide secret" : "Show secret"}
          aria-pressed={visible}
          onClick={() => setVisible((v) => !v)}
        >
          {visible ? <EyeOffIcon /> : <EyeIcon />}
        </InputGroupButton>
      </InputGroupAddon>
    </InputGroup>
  );
}
