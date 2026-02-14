"use client";

import { useEffect, useMemo, useState } from "react";

export type ExecResponse = {
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

export type ArtifactState = {
  key: string;
  path: string;
  exists: boolean;
  modified_at?: string;
};

export type StateResponse = {
  ok: boolean;
  workspace: string;
  runpack_path?: string;
  run_id?: string;
  manifest_digest?: string;
  trace_files?: string[];
  regress_result_path?: string;
  junit_path?: string;
  artifacts?: ArtifactState[];
  policy_paths?: string[];
  intent_paths?: string[];
  default_policy_path?: string;
  default_intent_path?: string;
  gait_config_exists: boolean;
  error?: string;
};

export const ACTIONS: Array<{ id: string; label: string; note: string }> = [
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

export function prettyAction(actionID: string): string {
  const action = ACTIONS.find((candidate) => candidate.id === actionID);
  return action?.label ?? actionID;
}

export function stateFallback(error: unknown): StateResponse {
  return {
    ok: false,
    workspace: "",
    gait_config_exists: false,
    error: error instanceof Error ? error.message : "unknown error",
  };
}

export function computeArtifactChanges(previousState: StateResponse | null, nextState: StateResponse): string[] {
  const previousArtifacts = new Map<string, ArtifactState>();
  for (const artifact of previousState?.artifacts ?? []) {
    previousArtifacts.set(artifact.key, artifact);
  }

  const changed: string[] = [];
  for (const artifact of nextState.artifacts ?? []) {
    const previous = previousArtifacts.get(artifact.key);
    const marker = `${artifact.key}${artifact.modified_at ? ` @ ${artifact.modified_at}` : ""}`;
    if (!previous) {
      changed.push(marker);
      continue;
    }
    if (previous.exists !== artifact.exists || previous.modified_at !== artifact.modified_at) {
      changed.push(marker);
    }
  }
  return changed;
}

export function buildActionArgs(actionID: string, runIDInput: string, policyPath: string, intentPath: string): Record<string, string> {
  const args: Record<string, string> = {};
  if (actionID === "regress_init") {
    args.run_id = runIDInput.trim();
  }
  if (actionID === "policy_block_test") {
    args.policy_path = policyPath;
    args.intent_path = intentPath;
  }
  return args;
}

export default function Page() {
  const [health, setHealth] = useState<"loading" | "ok" | "error">("loading");
  const [state, setState] = useState<StateResponse | null>(null);
  const [output, setOutput] = useState<ExecResponse | { error: string } | null>(null);
  const [running, setRunning] = useState<string | null>(null);
  const [runIDInput, setRunIDInput] = useState("run_demo");
  const [policyPath, setPolicyPath] = useState("");
  const [intentPath, setIntentPath] = useState("");
  const [lastAction, setLastAction] = useState<string | null>(null);
  const [lastRunAt, setLastRunAt] = useState<string | null>(null);
  const [changedArtifacts, setChangedArtifacts] = useState<string[]>([]);

  const workspaceSummary = useMemo(() => {
    if (!state) {
      return "Loading workspace...";
    }
    if (!state.ok) {
      return `State error: ${state.error ?? "unknown error"}`;
    }
    return `Workspace: ${state.workspace}`;
  }, [state]);

  const fetchState = async (): Promise<StateResponse> => {
    try {
      return await requestJSON<StateResponse>("/api/state");
    } catch (error) {
      return stateFallback(error);
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
      const initialState = await fetchState();
      setState(initialState);
      if (initialState.default_policy_path) {
        setPolicyPath(initialState.default_policy_path);
      } else if ((initialState.policy_paths?.length ?? 0) > 0) {
        setPolicyPath(initialState.policy_paths![0]);
      }
      if (initialState.default_intent_path) {
        setIntentPath(initialState.default_intent_path);
      } else if ((initialState.intent_paths?.length ?? 0) > 0) {
        setIntentPath(initialState.intent_paths![0]);
      }
      if (initialState.run_id) {
        setRunIDInput(initialState.run_id);
      }
    };
    void bootstrap();
  }, []);

  const runAction = async (actionID: string) => {
    setRunning(actionID);
    try {
      const args = buildActionArgs(actionID, runIDInput, policyPath, intentPath);
      const payload = await requestJSON<ExecResponse>("/api/exec", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command: actionID, args }),
      });
      const previousState = state;
      const refreshedState = await fetchState();
      setState(refreshedState);
      setChangedArtifacts(computeArtifactChanges(previousState, refreshedState));
      setLastAction(actionID);
      setLastRunAt(new Date().toISOString());
      setOutput(payload);
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
          <div className="status-grid">
            <div>
              <span className="status-label">Last Action</span>
              <strong>{lastAction ? prettyAction(lastAction) : "None yet"}</strong>
            </div>
            <div>
              <span className="status-label">Last Run At</span>
              <strong>{lastRunAt ?? "No runs yet"}</strong>
            </div>
            <div>
              <span className="status-label">Changed Artifacts</span>
              <strong>{changedArtifacts.length === 0 ? "None" : changedArtifacts.join(", ")}</strong>
            </div>
          </div>
          <div className="input-grid">
            <label>
              Regress Init run_id
              <input value={runIDInput} onChange={(event) => setRunIDInput(event.target.value)} disabled={running !== null} />
            </label>
            <label>
              Policy Fixture
              <select value={policyPath} onChange={(event) => setPolicyPath(event.target.value)} disabled={running !== null}>
                {(state?.policy_paths ?? []).map((value) => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Intent Fixture
              <select value={intentPath} onChange={(event) => setIntentPath(event.target.value)} disabled={running !== null}>
                {(state?.intent_paths ?? []).map((value) => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
              </select>
            </label>
          </div>
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
