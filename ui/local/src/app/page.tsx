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

type ActionTone = "runpack" | "regress" | "policy";

type ActionDefinition = {
  id: string;
  label: string;
  note: string;
  capability: string;
  tone: ActionTone;
};

export const DEFAULT_POLICY_PATH = "examples/policy/base_high_risk.yaml";
export const DEFAULT_INTENT_PATH = "examples/policy/intents/intent_delete.json";

export const ACTIONS: ActionDefinition[] = [
  {
    id: "demo",
    label: "1. Run Demo",
    note: "Create deterministic runpack artifacts.",
    capability: "Runpack",
    tone: "runpack",
  },
  {
    id: "verify_demo",
    label: "2. Verify",
    note: "Validate runpack integrity and manifest digest.",
    capability: "Runpack",
    tone: "runpack",
  },
  {
    id: "receipt_demo",
    label: "3. Ticket Footer",
    note: "Generate paste-ready verification receipt.",
    capability: "Runpack",
    tone: "runpack",
  },
  {
    id: "regress_init",
    label: "4. Regress Init",
    note: "Build deterministic fixture from a run_id.",
    capability: "Regress",
    tone: "regress",
  },
  {
    id: "regress_run",
    label: "5. Regress Run",
    note: "Run graders and emit JUnit report.",
    capability: "Regress",
    tone: "regress",
  },
  {
    id: "policy_block_test",
    label: "6. Policy Block",
    note: "Demonstrate fail-closed non-allow decision.",
    capability: "Policy",
    tone: "policy",
  },
];

export type JSONTokenKind = "plain" | "key" | "string" | "number" | "boolean" | "null" | "punct";

export type JSONToken = {
  text: string;
  kind: JSONTokenKind;
};

export type CommandTokenKind = "binary" | "subcommand" | "flag" | "value";

export type CommandToken = {
  text: string;
  kind: CommandTokenKind;
};

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

export function renderCommandPreview(actionID: string, runIDInput: string, policyPath: string, intentPath: string): string {
  const runID = runIDInput.trim() || "run_demo";
  const selectedPolicy = policyPath.trim() || DEFAULT_POLICY_PATH;
  const selectedIntent = intentPath.trim() || DEFAULT_INTENT_PATH;

  switch (actionID) {
    case "demo":
      return "gait demo --json";
    case "verify_demo":
      return "gait verify run_demo --json";
    case "receipt_demo":
      return "gait run receipt --from run_demo --json";
    case "regress_init":
      return `gait regress init --from ${runID} --json`;
    case "regress_run":
      return "gait regress run --json --junit ./gait-out/junit.xml";
    case "policy_block_test":
      return `gait policy test ${selectedPolicy} ${selectedIntent} --json`;
    default:
      return "gait --help";
  }
}

export function tokenizeCommand(command: string): CommandToken[] {
  const parts = command.trim().split(/\s+/).filter(Boolean);
  return parts.map((part, index) => {
    if (index === 0) {
      return { text: part, kind: "binary" };
    }
    if (part.startsWith("--")) {
      return { text: part, kind: "flag" };
    }
    if (index === 1) {
      return { text: part, kind: "subcommand" };
    }
    return { text: part, kind: "value" };
  });
}

export function tokenizeJSON(value: string): JSONToken[] {
  const tokenRegex =
    /("(?:\\.|[^"\\])*")(?=\s*:)|("(?:\\.|[^"\\])*")|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)|\b(true|false)\b|\bnull\b|([{}\[\],:])/g;
  const tokens: JSONToken[] = [];

  let lastIndex = 0;
  let match: RegExpExecArray | null = tokenRegex.exec(value);
  while (match !== null) {
    if (match.index > lastIndex) {
      tokens.push({ text: value.slice(lastIndex, match.index), kind: "plain" });
    }

    let kind: JSONTokenKind = "plain";
    if (match[1]) {
      kind = "key";
    } else if (match[2]) {
      kind = "string";
    } else if (match[3]) {
      kind = "number";
    } else if (match[4]) {
      kind = "boolean";
    } else if (match[0] === "null") {
      kind = "null";
    } else if (match[5]) {
      kind = "punct";
    }

    tokens.push({ text: match[0], kind });
    lastIndex = tokenRegex.lastIndex;
    match = tokenRegex.exec(value);
  }

  if (lastIndex < value.length) {
    tokens.push({ text: value.slice(lastIndex), kind: "plain" });
  }

  if (tokens.length === 0) {
    return [{ text: value, kind: "plain" }];
  }
  return tokens;
}

function toneClassName(tone: ActionTone): string {
  switch (tone) {
    case "runpack":
      return "tone-runpack";
    case "regress":
      return "tone-regress";
    case "policy":
      return "tone-policy";
    default:
      return "tone-runpack";
  }
}

export default function Page() {
  const [health, setHealth] = useState<"loading" | "ok" | "error">("loading");
  const [state, setState] = useState<StateResponse | null>(null);
  const [output, setOutput] = useState<ExecResponse | { error: string } | null>(null);
  const [running, setRunning] = useState<string | null>(null);
  const [selectedAction, setSelectedAction] = useState<string>(ACTIONS[0].id);
  const [runIDInput, setRunIDInput] = useState("run_demo");
  const [policyPath, setPolicyPath] = useState("");
  const [intentPath, setIntentPath] = useState("");
  const [lastAction, setLastAction] = useState<string | null>(null);
  const [lastRunAt, setLastRunAt] = useState<string | null>(null);
  const [changedArtifacts, setChangedArtifacts] = useState<string[]>([]);
  const [copied, setCopied] = useState<"command" | "output" | "state" | null>(null);

  const workspaceSummary = useMemo(() => {
    if (!state) {
      return "Loading workspace...";
    }
    if (!state.ok) {
      return `State error: ${state.error ?? "unknown error"}`;
    }
    return `Workspace: ${state.workspace}`;
  }, [state]);

  const commandPreview = useMemo(() => {
    return renderCommandPreview(selectedAction, runIDInput, policyPath, intentPath);
  }, [selectedAction, runIDInput, policyPath, intentPath]);

  const outputText = useMemo(() => {
    if (!output) {
      return "{\n  \"status\": \"ready\",\n  \"message\": \"Run a scenario to view command output.\"\n}";
    }
    return JSON.stringify(output, null, 2);
  }, [output]);

  const stateText = useMemo(() => {
    if (!state) {
      return "{\n  \"status\": \"loading\"\n}";
    }
    return JSON.stringify(state, null, 2);
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
    setSelectedAction(actionID);
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

  const copyText = async (kind: "command" | "output" | "state", value: string) => {
    if (!navigator.clipboard) {
      return;
    }
    await navigator.clipboard.writeText(value);
    setCopied(kind);
    window.setTimeout(() => setCopied((previous) => (previous === kind ? null : previous)), 1200);
  };

  const runDisabled = running !== null;

  return (
    <div className="page-shell">
      <header className="topbar">
        <div>
          <h1>Gait Playground</h1>
          <p>Local-first guided flows for runpack, regress, and policy validation.</p>
        </div>
        <div className="topbar-right">
          <div className={`health-pill health-${health}`}>{health === "loading" ? "Checking" : health === "ok" ? "Healthy" : "Unavailable"}</div>
          <span className="workspace-pill">{workspaceSummary}</span>
        </div>
      </header>

      <main className="layout-grid">
        <section className="panel">
          <div className="panel-header">
            <h2>Scenario Flow</h2>
            <span className="panel-note">Click any step to run it.</span>
          </div>
          <div className="scenario-list">
            {ACTIONS.map((action) => (
              <article key={action.id} className={`scenario-card ${selectedAction === action.id ? "scenario-selected" : ""}`}>
                <div className="scenario-header">
                  <span className={`scenario-pill ${toneClassName(action.tone)}`}>{action.capability}</span>
                  <span className="scenario-id">{action.id}</span>
                </div>
                <strong>{action.label}</strong>
                <p>{action.note}</p>
                <div className="scenario-actions">
                  <button type="button" className="ghost-button" onClick={() => setSelectedAction(action.id)} disabled={runDisabled}>
                    {selectedAction === action.id ? "Selected" : "Select"}
                  </button>
                  <button type="button" className="run-button" onClick={() => void runAction(action.id)} disabled={runDisabled}>
                    {running === action.id ? "Running..." : "Run"}
                  </button>
                </div>
              </article>
            ))}
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <h2>Execution View</h2>
            <span className="panel-note">Preview exact command before running.</span>
          </div>

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
              <input value={runIDInput} onChange={(event) => setRunIDInput(event.target.value)} disabled={runDisabled} />
            </label>
            <label>
              Policy Fixture
              <select value={policyPath} onChange={(event) => setPolicyPath(event.target.value)} disabled={runDisabled}>
                {(state?.policy_paths ?? []).map((value) => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Intent Fixture
              <select value={intentPath} onChange={(event) => setIntentPath(event.target.value)} disabled={runDisabled}>
                {(state?.intent_paths ?? []).map((value) => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <div className="code-panel">
            <div className="code-panel-header">
              <span>Command Preview</span>
              <button type="button" className="copy-button" onClick={() => void copyText("command", commandPreview)}>
                {copied === "command" ? "Copied" : "Copy"}
              </button>
            </div>
            <pre className="command-block">
              {tokenizeCommand(commandPreview).map((token, index, list) => (
                <span key={`${token.text}-${index}`} className={`cmd-${token.kind}`}>
                  {token.text}
                  {index + 1 < list.length ? " " : ""}
                </span>
              ))}
            </pre>
          </div>

          <div className="code-panel">
            <div className="code-panel-header">
              <span>Command Output</span>
              <button type="button" className="copy-button" onClick={() => void copyText("output", outputText)}>
                {copied === "output" ? "Copied" : "Copy"}
              </button>
            </div>
            <pre className="code-block">
              {tokenizeJSON(outputText).map((token, index) => (
                <span key={`${token.kind}-${index}`} className={`json-${token.kind}`}>
                  {token.text}
                </span>
              ))}
            </pre>
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <h2>Workspace State</h2>
            <button type="button" className="copy-button" onClick={() => void copyText("state", stateText)}>
              {copied === "state" ? "Copied" : "Copy"}
            </button>
          </div>
          <pre className="code-block">
            {tokenizeJSON(stateText).map((token, index) => (
              <span key={`${token.kind}-${index}`} className={`json-${token.kind}`}>
                {token.text}
              </span>
            ))}
          </pre>
        </section>
      </main>
    </div>
  );
}
