package ui

import "time"

type Config struct {
	ExecutablePath string
	WorkDir        string
	CommandTimeout time.Duration
	Runner         Runner
}

type ExecRequest struct {
	Command string            `json:"command"`
	Args    map[string]string `json:"args,omitempty"`
}

type ExecResponse struct {
	OK         bool           `json:"ok"`
	Command    string         `json:"command"`
	Argv       []string       `json:"argv,omitempty"`
	ExitCode   int            `json:"exit_code"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	Stdout     string         `json:"stdout,omitempty"`
	Stderr     string         `json:"stderr,omitempty"`
	JSON       map[string]any `json:"json,omitempty"`
	Error      string         `json:"error,omitempty"`
}

type HealthResponse struct {
	OK      bool   `json:"ok"`
	Service string `json:"service"`
}

type StateResponse struct {
	OK               bool     `json:"ok"`
	Workspace        string   `json:"workspace"`
	RunpackPath      string   `json:"runpack_path,omitempty"`
	RunID            string   `json:"run_id,omitempty"`
	ManifestDigest   string   `json:"manifest_digest,omitempty"`
	TraceFiles       []string `json:"trace_files,omitempty"`
	RegressResult    string   `json:"regress_result_path,omitempty"`
	JUnitPath        string   `json:"junit_path,omitempty"`
	GaitConfigExists bool     `json:"gait_config_exists"`
	Error            string   `json:"error,omitempty"`
}

type runCommandSpec struct {
	Command string
	Argv    []string
}
