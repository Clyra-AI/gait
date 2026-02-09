import Link from 'next/link';
import type { Metadata } from 'next';
import { canonicalUrl } from '@/lib/site';

export const metadata: Metadata = {
  title: 'Gait | Agent Control and Proof',
  description:
    'Gait is the offline-first execution boundary for production AI agents: runpack, regress, gate, doctor.',
  alternates: {
    canonical: canonicalUrl('/'),
  },
};

const QUICKSTART = `curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/install.sh | bash\ngait demo\ngait verify run_demo\ngait regress init --from run_demo --json\ngait regress run --json --junit ./gait-out/junit.xml`;

const features = [
  {
    title: 'Runpack: Verifiable Incident Evidence',
    description: 'Capture signed, deterministic execution artifacts you can verify offline and paste into tickets.',
    href: '/docs/concepts/mental_model',
  },
  {
    title: 'Regress: Incident -> Never Again',
    description: 'Turn a runpack into deterministic CI regressions with machine-readable output and JUnit.',
    href: '/docs/ci_regress_kit',
  },
  {
    title: 'Gate: Fail-Closed Tool Control',
    description: 'Evaluate structured tool-call intent against YAML policy and approval constraints.',
    href: '/docs/policy_rollout',
  },
  {
    title: 'Doctor: First 5 Minutes',
    description: 'Diagnose environment issues quickly with stable JSON and explicit fix guidance.',
    href: '/docs/uat_functional_plan',
  },
  {
    title: 'Vendor-Neutral Integrations',
    description: 'One wrapper, one sidecar, one CI lane across OpenAI Agents, LangChain, Autogen, OpenClaw, and AutoGPT.',
    href: '/docs/integration_checklist',
  },
  {
    title: 'Security and Contracts',
    description: 'Stable artifacts, explicit schemas, skill provenance, and endpoint action taxonomy.',
    href: '/docs/contracts/primitive_contract',
  },
];

export default function HomePage() {
  return (
    <div className="not-prose">
      <div className="text-center py-12 lg:py-20">
        <h1 className="text-4xl lg:text-6xl font-bold text-white mb-6">
          Control and Prove
          <span className="bg-gradient-to-r from-cyan-400 to-blue-500 bg-clip-text text-transparent"> Agent Actions</span>
        </h1>
        <p className="text-xl text-gray-400 max-w-3xl mx-auto mb-8">
          Gait turns production agent behavior into a governable execution substrate: signed runpacks, deterministic regressions,
          and fail-closed policy gates at the tool boundary.
        </p>
        <div className="flex flex-col sm:flex-row gap-4 justify-center">
          <Link href="/docs/install" className="px-6 py-3 bg-cyan-500 hover:bg-cyan-400 text-gray-900 font-semibold rounded-lg transition-colors">
            Start Here
          </Link>
          <Link href="/docs/integration_checklist" className="px-6 py-3 bg-gray-800 hover:bg-gray-700 text-gray-100 font-semibold rounded-lg border border-gray-700 transition-colors">
            Integrate in 30-120 Minutes
          </Link>
          <Link href="/llms" className="px-6 py-3 bg-gray-800 hover:bg-gray-700 text-gray-100 font-semibold rounded-lg border border-gray-700 transition-colors">
            LLM Context
          </Link>
        </div>
      </div>

      <div className="max-w-3xl mx-auto mb-16">
        <div className="bg-gray-800/50 rounded-lg border border-gray-700 p-4 overflow-x-auto">
          <pre><code className="text-cyan-300 text-sm">{QUICKSTART}</code></pre>
        </div>
      </div>

      <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6 mb-16">
        {features.map((feature) => (
          <Link
            key={feature.title}
            href={feature.href}
            className="block p-6 bg-gray-800/30 hover:bg-gray-800/50 rounded-lg border border-gray-700 hover:border-gray-600 transition-colors"
          >
            <h3 className="text-lg font-semibold text-white mb-2">{feature.title}</h3>
            <p className="text-sm text-gray-400">{feature.description}</p>
          </Link>
        ))}
      </div>

      <div className="mb-16 overflow-x-auto">
        <h2 className="text-2xl font-bold text-white mb-6 text-center">Why Teams Adopt Gait</h2>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-700">
              <th className="text-left py-3 px-4 text-gray-400"></th>
              <th className="text-left py-3 px-4 text-gray-400">Without Gait</th>
              <th className="text-left py-3 px-4 text-cyan-400">With Gait</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            <tr>
              <td className="py-3 px-4 text-gray-300 font-medium">Incident evidence</td>
              <td className="py-3 px-4 text-gray-500">logs + screenshots</td>
              <td className="py-3 px-4 text-gray-300">signed runpack + ticket footer</td>
            </tr>
            <tr>
              <td className="py-3 px-4 text-gray-300 font-medium">Regression loop</td>
              <td className="py-3 px-4 text-gray-500">manual repro, often skipped</td>
              <td className="py-3 px-4 text-gray-300">deterministic fixture + CI lane</td>
            </tr>
            <tr>
              <td className="py-3 px-4 text-gray-300 font-medium">High-risk tool calls</td>
              <td className="py-3 px-4 text-gray-500">best-effort guardrails</td>
              <td className="py-3 px-4 text-gray-300">fail-closed policy + approvals</td>
            </tr>
            <tr>
              <td className="py-3 px-4 text-gray-300 font-medium">Audit posture</td>
              <td className="py-3 px-4 text-gray-500">incomplete reconstruction</td>
              <td className="py-3 px-4 text-gray-300">offline verifiable artifacts</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div className="text-center py-12 border-t border-gray-800">
        <h2 className="text-2xl font-bold text-white mb-4">Start with one command. Keep the evidence forever.</h2>
        <p className="text-gray-400 mb-6">Read the install and integration checklist, then wire your first policy-gated tool boundary.</p>
        <Link href="/docs/install" className="inline-block px-6 py-3 bg-cyan-500 hover:bg-cyan-400 text-gray-900 font-semibold rounded-lg transition-colors">
          Open Install Guide
        </Link>
      </div>
    </div>
  );
}
