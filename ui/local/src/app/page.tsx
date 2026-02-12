"use client";

import { useEffect, useMemo, useState } from "react";

type ExecResponse = {
  ok: boolean;
  command: string;
  argv?: string[];
  exit_code: number;
  duration_ms?: number;
  stdout?: string;
  stderr?: string;
  json?: Record<string, unknown>;
  error?: string;
};

type StateResponse = {
  ok: boolean;
  workspace: string;
  runpack_path?: string;
  run_id?: string;
  manifest_digest?: string;
  trace_files?: string[];
  regress_result_path?: string;
  junit_path?: string;
  gait_config_exists: boolean;
  error?: string;
};

const ACTIONS: Array<{ id: string; label: string; note: string }> = [
  { id: "demo", label: "1. Run Demo", note: "Create deterministic runpack" },
  { id: "verify_demo", label: "2. Verify", note: "Validate artifact integrity" },
  { id: "receipt_demo", label: "3. Ticket Footer", note: "Extract paste-ready proof" },
  { id: "regress_init", label: "4. Regress Init", note: "Create fixture from incident" },
  { id: "regress_run", label: "5. Regress Run", note: "Execute deterministic graders" },
  { id: "policy_block_test", label: "6. Policy Block", note: "Show fail-closed non-allow" },
];

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init);
  const text = await response.text();
  let payload: unknown;
  try {
    payload = JSON.parse(text);
  } catch {
    throw new Error(`invalid JSON from ${path}: ${text}`);
  }
  if (!response.ok) {
    throw new Error(`request failed (${response.status}): ${JSON.stringify(payload)}`);
  }
  return payload as T;
}

export default function Page() {
  const [health, setHealth] = useState<"loading" | "ok" | "error">("loading");
  const [state, setState] = useState<StateResponse | null>(null);
  const [output, setOutput] = useState<ExecResponse | { error: string } | null>(null);
  const [running, setRunning] = useState<string | null>(null);

  const workspaceSummary = useMemo(() => {
    if (!state) {
      return "Loading workspace...";
    }
    if (!state.ok) {
      return `State error: ${state.error ?? "unknown error"}`;
    }
    return `Workspace: ${state.workspace}`;
  }, [state]);

  const refreshState = async () => {
    try {
      const payload = await requestJSON<StateResponse>("/api/state");
      setState(payload);
    } catch (error) {
      setState({
        ok: false,
        workspace: "",
        gait_config_exists: false,
        error: error instanceof Error ? error.message : "unknown error",
      });
    }
  };

  useEffect(() => {
    const bootstrap = async () => {
      try {
        await requestJSON<{ ok: boolean }>("/api/health");
        setHealth("ok");
      } catch {
        setHealth("error");
      }
      await refreshState();
    };
    void bootstrap();
  }, []);

  const runAction = async (actionID: string) => {
    setRunning(actionID);
    try {
      const payload = await requestJSON<ExecResponse>("/api/exec", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command: actionID }),
      });
      setOutput(payload);
      await refreshState();
    } catch (error) {
      setOutput({ error: error instanceof Error ? error.message : "unknown error" });
    } finally {
      setRunning(null);
    }
  };

  return (
    <div className="page-shell">
      <header className="topbar">
        <div>
          <h1>Gait Local UI</h1>
          <p>Operator command center for first-run adoption and deterministic proof.</p>
        </div>
        <div className={`health-pill health-${health}`}>{health === "loading" ? "Checking..." : health === "ok" ? "Healthy" : "Unavailable"}</div>
      </header>

      <main className="layout-grid">
        <section className="panel">
          <h2>15-Minute Flow</h2>
          <p className="muted">{workspaceSummary}</p>
          <div className="action-grid">
            {ACTIONS.map((action) => (
              <button key={action.id} onClick={() => void runAction(action.id)} disabled={running !== null}>
                <strong>{action.label}</strong>
                <span>{running === action.id ? "Running..." : action.note}</span>
              </button>
            ))}
          </div>
          <pre className="code-block">{JSON.stringify(output, null, 2) || "Run an action to view command output."}</pre>
        </section>

        <section className="panel">
          <h2>Workspace State</h2>
          <pre className="code-block">{JSON.stringify(state, null, 2) || "Loading..."}</pre>
        </section>
      </main>
    </div>
  );
}
