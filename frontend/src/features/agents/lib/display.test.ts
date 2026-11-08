import { describe, expect, it } from "vitest";

import { formatProbeTime, transportHost } from "./display";

describe("transportHost", () => {
  it("returns the host of the transport URL, with its port", () => {
    expect(transportHost("wss://agent.example:8443/ws/{{var.id}}")).toBe("agent.example:8443");
    expect(transportHost("https://api.example/chat")).toBe("api.example");
  });

  it("falls back to the raw value when it is not a URL", () => {
    expect(transportHost("not a url")).toBe("not a url");
    expect(transportHost("")).toBe("");
  });
});

describe("formatProbeTime", () => {
  it("shows milliseconds since the start as seconds with two decimals", () => {
    expect(formatProbeTime(0)).toBe("0.00s");
    expect(formatProbeTime(1234)).toBe("1.23s");
    expect(formatProbeTime(12000)).toBe("12.00s");
  });
});
