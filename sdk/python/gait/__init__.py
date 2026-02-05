from .adapter import AdapterOutcome, GateEnforcementError, ToolAdapter
from .client import (
    GaitCommandError,
    GaitError,
    capture_demo_runpack,
    capture_intent,
    create_regress_fixture,
    evaluate_gate,
    write_trace,
)
from .models import (
    DemoCapture,
    GateEvalResult,
    IntentArgProvenance,
    IntentContext,
    IntentRequest,
    IntentTarget,
    RegressInitResult,
    TraceRecord,
)

__all__ = [
    "__version__",
    "AdapterOutcome",
    "DemoCapture",
    "GaitCommandError",
    "GaitError",
    "GateEnforcementError",
    "GateEvalResult",
    "IntentArgProvenance",
    "IntentContext",
    "IntentRequest",
    "IntentTarget",
    "RegressInitResult",
    "ToolAdapter",
    "TraceRecord",
    "capture_demo_runpack",
    "capture_intent",
    "create_regress_fixture",
    "evaluate_gate",
    "write_trace",
]

__version__ = "0.0.0.dev0"
