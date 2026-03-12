import Link from 'next/link';
import type { Metadata } from 'next';
import { canonicalUrl } from '@/lib/site';

export const metadata: Metadata = {
  title: 'Gait | Policy-as-Code for Agent Tool Calls',
  description:
    'Gait is the offline-first policy-as-code runtime for AI agent tool calls: repo bootstrap with gait init/check, fail-closed tool-boundary verdicts, signed evidence, deterministic regressions, MCP trust, and durable jobs.',
  alternates: {
    canonical: canonicalUrl('/'),
  },
};

const QUICKSTART = `# Install
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash

# Bootstrap repo policy-as-code
gait init --json
gait check --json

# Create a signed artifact and verify it
gait demo
gait verify run_demo --json

# Turn it into a CI regression gate
gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml`;

const POLICY_SHAPE = `{
  "ok": true,
  "policy_path": ".gait.yaml",
  "default_verdict": "block",
  "rule_count": 7
}`;

const INTEGRATION_SNIPPET = `def dispatch_tool(tool_call):
    decision = gait_evaluate(tool_call)
    if decision["verdict"] != "allow":
        return {"executed": False, "verdict": decision["verdict"]}
    return {"executed": True, "result": execute_real_tool(tool_call)}`;

const features = [
  {
    title: 'Gate Before Tool Execution',
    description: 'Evaluate structured intent with fail-closed YAML policy. Any verdict other than allow is non-executing.',
    href: '/docs/policy_authoring',
  },
  {
    title: 'Signed Evidence You Can Reuse',
    description: 'Keep signed traces, runpacks, packs, and callpacks you can verify offline and attach to PRs, incidents, and audits.',
    href: '/docs/contracts/packspec_v1',
  },
  {
    title: 'Incident to CI Gate',
    description: 'Use gait capture, gait regress add, or gait regress bootstrap to turn a failure into a permanent regression with stable exit codes.',
    href: '/docs/ci_regress_kit',
  },
  {
    title: 'LangChain Middleware, Truthfully Scoped',
    description: 'The official LangChain lane is middleware with optional callback correlation. Enforcement still happens only at wrap_tool_call.',
    href: '/docs/sdk/python',
  },
  {
    title: 'MCP Trust Is Complementary',
    description: 'Use gait mcp verify on local trust snapshots before proxy or serve. External scanners find; Gait enforces.',
    href: '/docs/mcp_capability_matrix',
  },
  {
    title: 'Durable Jobs and Voice Stay Additive',
    description: 'Checkpointed jobs, voice commitment gating, and context evidence ride on the same artifact and policy contracts.',
    href: '/docs/flows',
  },
];

const faqs = [
  {
    question: 'What should teams run first?',
    answer:
      'Run gait init --json, gait check --json, gait demo, gait verify run_demo --json, then gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml.',
  },
  {
    question: 'Where does Gait enforce policy?',
    answer:
      'At the exact tool boundary where your runtime is about to execute a real side effect. Only allow executes. Block and require_approval stay non-executing.',
  },
  {
    question: 'What does Gait do that logs do not?',
    answer:
      'Gait produces signed traces and packs with deterministic verification, so incidents are portable, independently verifiable evidence rather than best-effort log interpretation.',
  },
  {
    question: 'Does Gait require a hosted service?',
    answer:
      'No. Core workflows are offline-first and run locally: capture, verify, diff, policy evaluation, regressions, and voice/context verification can run without a network dependency.',
  },
  {
    question: 'What problem does Gait solve for long-running agent work?',
    answer:
      'Multi-step and multi-hour agent jobs fail mid-flight, losing state and provenance. Gait dispatches durable jobs with checkpointed state, pause/resume/cancel, and deterministic stop reasons so work survives failures and stays auditable.',
  },
  {
    question: 'Can Gait gate voice agent actions?',
    answer:
      'Yes. Voice mode gates high-stakes spoken commitments before they are uttered. A signed SayToken capability token must be present for gated speech, and every call produces a signed callpack artifact.',
  },
  {
    question: 'What is context evidence?',
    answer:
      'Context evidence is deterministic proof of what context material the model was working from at decision time. Gait captures privacy-aware context envelopes and enforces fail-closed policy when evidence is missing for high-risk actions.',
  },
  {
    question: 'How do I turn a failed agent run into a CI gate?',
    answer:
      'Run gait regress bootstrap --from <run_id> --junit output.xml. This converts the run into a permanent regression fixture. Exit 0 means pass, exit 5 means drift.',
  },
];

const softwareApplicationJsonLd = {
  '@context': 'https://schema.org',
  '@type': 'SoftwareApplication',
  name: 'Gait',
  applicationCategory: 'DeveloperApplication',
  operatingSystem: 'Linux, macOS, Windows',
  description:
    'Offline-first policy-as-code runtime for AI agent tool calls: fail-closed tool-boundary policy, signed evidence, deterministic regressions, MCP trust preflight, durable jobs, and voice/context proof.',
  url: 'https://clyra-ai.github.io/gait/',
  softwareHelp: 'https://clyra-ai.github.io/gait/docs/',
  codeRepository: 'https://github.com/Clyra-AI/gait',
  offers: {
    '@type': 'Offer',
    price: '0',
    priceCurrency: 'USD',
  },
};

const faqJsonLd = {
  '@context': 'https://schema.org',
  '@type': 'FAQPage',
  mainEntity: faqs.map((entry) => ({
    '@type': 'Question',
    name: entry.question,
    acceptedAnswer: {
      '@type': 'Answer',
      text: entry.answer,
    },
  })),
};

export default function HomePage() {
  return (
    <div className="not-prose">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(softwareApplicationJsonLd) }} />
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(faqJsonLd) }} />

      <div className="py-12 text-center lg:py-20">
        <h1 className="mb-6 text-4xl font-bold text-white lg:text-6xl">
          Policy-as-Code for Agent Tool Calls.
          <span className="bg-gradient-to-r from-cyan-400 to-blue-500 bg-clip-text text-transparent"> Portable proof for every verdict.</span>
        </h1>
        <p className="mx-auto mb-8 max-w-3xl text-xl text-gray-400">
          Bootstrap repo policy with <code>gait init</code> and <code>gait check</code>, evaluate structured intent before real tool execution,
          and turn incidents into deterministic CI regressions without a hosted control plane.
        </p>
        <div className="flex flex-col justify-center gap-4 sm:flex-row">
          <Link href="/docs/install" className="rounded-lg bg-cyan-500 px-6 py-3 font-semibold text-gray-900 transition-colors hover:bg-cyan-400">
            Start Here
          </Link>
          <Link
            href="/docs/integration_checklist"
            className="rounded-lg border border-gray-700 bg-gray-800 px-6 py-3 font-semibold text-gray-100 transition-colors hover:bg-gray-700"
          >
            Integration Checklist
          </Link>
        </div>
      </div>

      <div className="mx-auto mb-16 max-w-3xl">
        <h2 className="mb-4 text-center text-2xl font-bold text-white">Put Gait at the Tool Boundary</h2>
        <p className="mb-4 text-center text-sm text-gray-300">
          Normalize intent, evaluate a verdict, execute side effects only on <code>allow</code>, and keep signed trace artifacts.
        </p>
        <div className="mb-5 grid gap-4 text-sm md:grid-cols-3">
          <div className="rounded-lg border border-gray-700 bg-gray-900/40 p-4">
            <h3 className="mb-2 font-semibold text-white">Inline Wrapper</h3>
            <p className="text-gray-300">Call <code>gait gate eval</code> in your dispatcher before real tool execution.</p>
          </div>
          <div className="rounded-lg border border-gray-700 bg-gray-900/40 p-4">
            <h3 className="mb-2 font-semibold text-white">LangChain Middleware</h3>
            <p className="text-gray-300">Official middleware with optional callback correlation. Enforcement still happens in <code>wrap_tool_call</code>.</p>
          </div>
          <div className="rounded-lg border border-gray-700 bg-gray-900/40 p-4">
            <h3 className="mb-2 font-semibold text-white">MCP Boundary</h3>
            <p className="text-gray-300">Preflight server trust with <code>gait mcp verify</code>, then use <code>gait mcp proxy</code> or <code>gait mcp serve</code>.</p>
          </div>
        </div>
        <div className="mb-5 overflow-x-auto rounded-lg border border-gray-700 bg-gray-900/60 p-4">
          <pre>
            <code className="text-sm text-cyan-300">{INTEGRATION_SNIPPET}</code>
          </pre>
        </div>
        <div className="mb-4 overflow-x-auto rounded-lg border border-gray-700 bg-gray-800/50 p-4">
          <pre>
            <code className="text-sm text-cyan-300">{QUICKSTART}</code>
          </pre>
        </div>
        <div className="overflow-x-auto rounded-lg border border-gray-700 bg-gray-900/60 p-4">
          <pre>
            <code className="text-sm text-emerald-300">{POLICY_SHAPE}</code>
          </pre>
        </div>
        <p className="mt-3 text-xs text-gray-500">
          Start with <Link href="/docs/integration_checklist" className="text-cyan-300 hover:text-cyan-200">integration checklist</Link>,{' '}
          <Link href="/docs/agent_integration_boundary" className="text-cyan-300 hover:text-cyan-200">boundary guide</Link>, and{' '}
          <Link href="/docs/sdk/python" className="text-cyan-300 hover:text-cyan-200">Python SDK contract</Link>. The example JSON shape above matches a real{' '}
          <code>gait check --json</code> run.
        </p>
      </div>

      <div className="mb-16 grid gap-6 md:grid-cols-2 lg:grid-cols-3">
        {features.map((feature) => (
          <Link
            key={feature.title}
            href={feature.href}
            className="block rounded-lg border border-gray-700 bg-gray-800/30 p-6 transition-colors hover:border-gray-600 hover:bg-gray-800/50"
          >
            <h3 className="mb-2 text-lg font-semibold text-white">{feature.title}</h3>
            <p className="text-sm text-gray-400">{feature.description}</p>
          </Link>
        ))}
      </div>

      <div className="mb-16 overflow-x-auto">
        <h2 className="mb-6 text-center text-2xl font-bold text-white">Why Teams Adopt Gait</h2>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-700">
              <th className="px-4 py-3 text-left text-gray-400"></th>
              <th className="px-4 py-3 text-left text-gray-400">Without Gait</th>
              <th className="px-4 py-3 text-left text-cyan-400">With Gait</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            <tr>
              <td className="px-4 py-3 font-medium text-gray-300">Tool-call control</td>
              <td className="px-4 py-3 text-gray-500">best-effort prompt checks</td>
              <td className="px-4 py-3 text-gray-300">fail-closed structured verdicts at execution time</td>
            </tr>
            <tr>
              <td className="px-4 py-3 font-medium text-gray-300">Incident evidence</td>
              <td className="px-4 py-3 text-gray-500">logs + screenshots</td>
              <td className="px-4 py-3 text-gray-300">signed trace or pack + ticket footer</td>
            </tr>
            <tr>
              <td className="px-4 py-3 font-medium text-gray-300">Regression loop</td>
              <td className="px-4 py-3 text-gray-500">manual repro, often skipped</td>
              <td className="px-4 py-3 text-gray-300">deterministic fixture + CI gate</td>
            </tr>
            <tr>
              <td className="px-4 py-3 font-medium text-gray-300">MCP trust</td>
              <td className="px-4 py-3 text-gray-500">ad hoc server trust decisions</td>
              <td className="px-4 py-3 text-gray-300">local snapshot preflight + policy enforcement</td>
            </tr>
            <tr>
              <td className="px-4 py-3 font-medium text-gray-300">Long-running agent work</td>
              <td className="px-4 py-3 text-gray-500">fails mid-flight, lost state</td>
              <td className="px-4 py-3 text-gray-300">durable jobs with checkpoints + resume</td>
            </tr>
            <tr>
              <td className="px-4 py-3 font-medium text-gray-300">Voice commitments</td>
              <td className="px-4 py-3 text-gray-500">hope they say the right thing</td>
              <td className="px-4 py-3 text-gray-300">gated before speech + signed callpack</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div className="mb-16">
        <h2 className="mb-6 text-center text-2xl font-bold text-white">Frequently Asked Questions</h2>
        <div className="grid gap-4 md:grid-cols-2">
          {faqs.map((entry) => (
            <div key={entry.question} className="rounded-lg border border-gray-700 bg-gray-900/40 p-5">
              <h3 className="mb-2 text-base font-semibold text-gray-100">{entry.question}</h3>
              <p className="text-sm text-gray-300">{entry.answer}</p>
            </div>
          ))}
        </div>
      </div>

      <div className="border-t border-gray-800 py-12 text-center">
        <h2 className="mb-4 text-2xl font-bold text-white">Start with policy bootstrap. Add evidence, CI, MCP trust, and jobs as needed.</h2>
        <p className="mb-6 text-gray-400">The first five commands are real: init, check, demo, verify, regress bootstrap.</p>
        <Link href="/docs/install" className="inline-block rounded-lg bg-cyan-500 px-6 py-3 font-semibold text-gray-900 transition-colors hover:bg-cyan-400">
          Open Install Guide
        </Link>
        <p className="mt-5 text-sm text-gray-500">
          For assistant and crawler discovery resources, use{' '}
          <Link href="/llms" className="text-cyan-300 hover:text-cyan-200">
            LLM Context
          </Link>
          .
        </p>
      </div>
    </div>
  );
}
