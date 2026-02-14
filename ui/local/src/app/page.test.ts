import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import Page, {
  buildActionArgs,
  computeArtifactChanges,
  prettyAction,
  renderCommandPreview,
  stateFallback,
  tokenizeCommand,
  tokenizeJSON,
  type StateResponse,
} from "./page";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "content-type": "application/json",
    },
  });
}

async function waitFor(assertion: () => void, timeoutMS = 1200): Promise<void> {
  const startedAt = Date.now();
  while (true) {
    try {
      assertion();
      return;
    } catch (error) {
      if (Date.now() - startedAt > timeoutMS) {
        throw error;
      }
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 25));
      });
    }
  }
}

describe("ui page helpers", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    vi.restoreAllMocks();
  });

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

  test("renderCommandPreview mirrors backend command intent", () => {
    expect(renderCommandPreview("demo", "run_demo", "", "")).toBe("gait demo --json");
    expect(renderCommandPreview("regress_init", "  run_42  ", "", "")).toBe("gait regress init --from run_42 --json");
    expect(renderCommandPreview("policy_block_test", "run_demo", "policy.yaml", "intent.json")).toBe(
      "gait policy test policy.yaml intent.json --json",
    );
  });

  test("tokenizeCommand highlights binary, flags, and values", () => {
    const tokens = tokenizeCommand("gait regress run --json --junit ./gait-out/junit.xml");
    expect(tokens[0]).toEqual({ text: "gait", kind: "binary" });
    expect(tokens[1]).toEqual({ text: "regress", kind: "subcommand" });
    expect(tokens[3]).toEqual({ text: "--json", kind: "flag" });
    expect(tokens[5]).toEqual({ text: "./gait-out/junit.xml", kind: "value" });
  });

  test("tokenizeJSON classifies keys and literals", () => {
    const tokens = tokenizeJSON('{"ok":true,"count":2,"msg":"done","extra":null}');
    const kinds = tokens.map((token) => token.kind);
    expect(kinds).toContain("key");
    expect(kinds).toContain("boolean");
    expect(kinds).toContain("number");
    expect(kinds).toContain("string");
    expect(kinds).toContain("null");
  });

  test("page renders flow and runs demo action", async () => {
    const initialState: StateResponse = {
      ok: true,
      workspace: "/tmp/work",
      gait_config_exists: true,
      run_id: "run_demo",
      artifacts: [{ key: "runpack", path: "gait-out/runpack_run_demo.zip", exists: false }],
      policy_paths: ["examples/policy/base_high_risk.yaml"],
      intent_paths: ["examples/policy/intents/intent_delete.json"],
      default_policy_path: "examples/policy/base_high_risk.yaml",
      default_intent_path: "examples/policy/intents/intent_delete.json",
    };

    const nextState: StateResponse = {
      ...initialState,
      artifacts: [
        {
          key: "runpack",
          path: "gait-out/runpack_run_demo.zip",
          exists: true,
          modified_at: "2026-02-14T00:00:02Z",
        },
      ],
    };

    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({ ok: true }))
      .mockResolvedValueOnce(jsonResponse(initialState))
      .mockResolvedValueOnce(
        jsonResponse({
          ok: true,
          command: "demo",
          exit_code: 0,
          stdout: "{}",
        }),
      )
      .mockResolvedValueOnce(jsonResponse(nextState));

    vi.stubGlobal("fetch", fetchMock);

    await act(async () => {
      root.render(createElement(Page));
    });

    await waitFor(() => {
      expect(container.textContent).toContain("Gait Playground");
      expect(container.textContent).toContain("Healthy");
      expect(container.textContent).toContain("Workspace: /tmp/work");
      expect(container.textContent).toContain("gait demo --json");
    });

    const runButtons = container.querySelectorAll(".run-button");
    expect(runButtons.length).toBeGreaterThan(0);

    await act(async () => {
      (runButtons[0] as HTMLButtonElement).click();
    });

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/exec",
        expect.objectContaining({ method: "POST" }),
      );
      expect(container.textContent).toContain("Changed Artifacts");
      expect(container.textContent).toContain("runpack @ 2026-02-14T00:00:02Z");
    });
  });
});
