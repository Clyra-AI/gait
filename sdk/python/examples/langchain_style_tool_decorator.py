from __future__ import annotations

from gait import IntentContext, ToolAdapter, gate_tool

adapter = ToolAdapter(policy_path="gait.policy.yaml", gait_bin="gait")


@gate_tool(
    adapter=adapter,
    context=IntentContext(identity="agent-langchain", workspace="/srv/agent", risk_class="high"),
    tool_name="tool.search",
    trace_out="./gait-out/trace_langchain_search.json",
)
def search_docs(query: str, top_k: int = 3) -> str:
    # Replace this with your retrieval implementation.
    return f"top_{top_k}_results_for:{query}"


if __name__ == "__main__":
    print(search_docs("gait policy rollout"))
