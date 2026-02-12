from .adapter import AdapterOutcome, GateEnforcementError, ToolAdapter
from .client import (
    GaitCommandError,
    GaitError,
    capture_demo_runpack,
    capture_intent,
    create_regress_fixture,
    evaluate_gate,
    record_runpack,
    write_trace,
)
from .decorators import gate_tool
from .models import (
    DelegationLink,
    DemoCapture,
    GateEvalResult,
    IntentArgProvenance,
    IntentContext,
    IntentDelegation,
    IntentRequest,
    IntentTarget,
    RegressInitResult,
    RunRecordCapture,
    TraceRecord,
)
from .session import RunAttempt, RunSession, run_session

__all__ = [
    "__version__",
    "AdapterOutcome",
    "DemoCapture",
    "DelegationLink",
    "GaitCommandError",
    "GaitError",
    "GateEnforcementError",
    "GateEvalResult",
    "IntentArgProvenance",
    "IntentContext",
    "IntentDelegation",
    "IntentRequest",
    "IntentTarget",
    "RegressInitResult",
    "RunAttempt",
    "RunRecordCapture",
    "RunSession",
    "ToolAdapter",
    "TraceRecord",
    "capture_demo_runpack",
    "capture_intent",
    "create_regress_fixture",
    "evaluate_gate",
    "gate_tool",
    "record_runpack",
    "run_session",
    "write_trace",
]

__version__ = "0.0.0.dev0"
