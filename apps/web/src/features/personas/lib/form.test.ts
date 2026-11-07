import { describe, expect, it } from "vitest";

import type { Capability, Persona } from "@/lib/api/types";

import {
  buildPersonaInput,
  initialPersonaValues,
  mapPersonaFieldErrors,
  personaFormValues,
  TEMPERAMENTS,
  voiceConnections,
} from "./form";

const persona: Persona = {
  id: "p1",
  name: "Hurried Marta",
  locale: "es-MX",
  temperament: "hurried",
  notes: "Calls from the car.",
  voice: { connectionId: "c-tts", voice: "es-MX-DaliaNeural" },
  scenarioCount: 2,
  createdAt: "2026-11-07T10:00:00Z",
  updatedAt: "2026-11-07T10:00:00Z",
};

describe("persona form values", () => {
  it("starts a polite en-US persona without a voice", () => {
    expect(initialPersonaValues()).toEqual({
      name: "",
      locale: "en-US",
      temperament: "polite",
      notes: "",
      voice: { connectionId: "", voice: "" },
    });
  });

  it("round-trips a stored persona", () => {
    expect(buildPersonaInput(personaFormValues(persona))).toEqual({
      name: "Hurried Marta",
      locale: "es-MX",
      temperament: "hurried",
      notes: "Calls from the car.",
      voice: { connectionId: "c-tts", voice: "es-MX-DaliaNeural" },
    });
  });

  it("sends a null voice when no connection is picked and trims text", () => {
    const values = { ...initialPersonaValues(), name: "  Ana ", locale: " es-ES ", notes: " x ", voice: { connectionId: "", voice: "alloy" } };
    expect(buildPersonaInput(values)).toEqual({ name: "Ana", locale: "es-ES", temperament: "polite", notes: "x", voice: null });
  });

  it("describes every temperament", () => {
    expect(TEMPERAMENTS.map((t) => t.value)).toEqual([
      "polite", "hurried", "skeptical", "confused", "angry", "friendly", "calm", "detailed",
    ]);
    for (const t of TEMPERAMENTS) expect(t.description.length).toBeGreaterThan(10);
  });
});

describe("voiceConnections", () => {
  it("keeps connections with tts or realtime capability", () => {
    const connections: { id: string; capabilities: Capability[] }[] = [
      { id: "a", capabilities: ["llm"] },
      { id: "b", capabilities: ["tts", "stt"] },
      { id: "c", capabilities: ["realtime", "llm"] },
    ];
    expect(voiceConnections(connections).map((c) => c.id)).toEqual(["b", "c"]);
  });
});

describe("mapPersonaFieldErrors", () => {
  it("maps voice paths onto the voice inputs and leaves the rest unmatched", () => {
    expect(
      mapPersonaFieldErrors({
        name: "Taken",
        "voice.connectionId": "Needs tts",
        "voice.voice": "Too long",
        voice: "Invalid",
        createdAt: "odd",
      }),
    ).toEqual({
      matched: [
        ["name", "Taken"],
        ["voice.connectionId", "Needs tts"],
        ["voice.voice", "Too long"],
        ["voice.connectionId", "Invalid"],
      ],
      unmatched: ["odd"],
    });
  });
});
