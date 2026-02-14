import { describe, expect, test } from "vitest";
import { buildActionArgs, computeArtifactChanges, prettyAction, stateFallback, type StateResponse } from "./page";

describe("ui page helpers", () => {
  test("prettyAction resolves known labels", () => {
    expect(prettyAction("demo")).toContain("Run Demo");
    expect(prettyAction("unknown")).toBe("unknown");
  });

  test("stateFallback normalizes errors", () => {
    const fromError = stateFallback(new Error("network down"));
    expect(fromError.ok).toBe(false);
    expect(fromError.error).toBe("network down");

    const fromUnknown = stateFallback("boom");
    expect(fromUnknown.error).toBe("unknown error");
  });

  test("buildActionArgs wires regress and policy transitions", () => {
    expect(buildActionArgs("demo", "run_demo", "p.yaml", "i.json")).toEqual({});
    expect(buildActionArgs("regress_init", "  run_123  ", "p.yaml", "i.json")).toEqual({
      run_id: "run_123",
    });
    expect(buildActionArgs("policy_block_test", "run_demo", "policy.yaml", "intent.json")).toEqual({
      policy_path: "policy.yaml",
      intent_path: "intent.json",
    });
  });

  test("computeArtifactChanges returns added and modified artifacts", () => {
    const previousState: StateResponse = {
      ok: true,
      workspace: "/tmp/work",
      gait_config_exists: false,
      artifacts: [
        { key: "runpack", path: "a.zip", exists: true, modified_at: "2026-02-14T00:00:00Z" },
        { key: "junit", path: "junit.xml", exists: false },
      ],
    };

    const nextState: StateResponse = {
      ok: true,
      workspace: "/tmp/work",
      gait_config_exists: false,
      artifacts: [
        { key: "runpack", path: "a.zip", exists: true, modified_at: "2026-02-14T00:00:02Z" },
        { key: "junit", path: "junit.xml", exists: true, modified_at: "2026-02-14T00:00:02Z" },
        { key: "regress_result", path: "regress_result.json", exists: true, modified_at: "2026-02-14T00:00:02Z" },
      ],
    };

    const changed = computeArtifactChanges(previousState, nextState);
    expect(changed).toContain("runpack @ 2026-02-14T00:00:02Z");
    expect(changed).toContain("junit @ 2026-02-14T00:00:02Z");
    expect(changed).toContain("regress_result @ 2026-02-14T00:00:02Z");
  });
});
