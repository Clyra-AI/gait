import Link from 'next/link';
import type { Metadata } from 'next';
import { canonicalUrl } from '@/lib/site';

export const metadata: Metadata = {
  title: 'Gait | Durable Agent Runtime with Signed Proof',
  description:
    'Gait is an offline-first runtime for production AI agents: durable jobs, signed packs, voice agent gating, context evidence, deterministic regressions, and fail-closed policy gates.',
  alternates: {
    canonical: canonicalUrl('/'),
  },
};

const QUICKSTART = `# Install
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash

# Create a signed pack from a synthetic agent run
gait demo

# Prove it's intact
gait verify run_demo

# Turn it into a CI regression gate
gait regress bootstrap --from run_demo --junit ./gait-out/junit.xml`;

const features = [
  {
    title: 'Durable Jobs: Run Without Losing State',
    description: 'Dispatch multi-step, multi-hour agent work with checkpoints, pause/resume/cancel, approval gates, and deterministic stop reasons.',
    href: '/docs/flows',
  },
  {
    title: 'Signed Packs: Portable Proof',
    description: 'Every run and job emits a signed pack you can verify, diff, and inspect offline. Attach it to PRs, incidents, and audits.',
    href: '/docs/contracts/packspec_v1',
  },
  {
    title: 'Regress: Incident to CI Gate',
    description: 'One command converts a failure into a permanent regression test with JUnit output and stable exit codes.',
    href: '/docs/ci_regress_kit',
  },
  {
    title: 'Gate: Fail-Closed Policy Enforcement',
    description: 'Evaluate structured tool-call intent against policy before side effects execute. Non-allow means non-execute.',
    href: '/docs/policy_authoring',
  },
  {
    title: 'Voice Agent Gating',
    description: 'Gate high-stakes spoken commitments before they are uttered. Signed SayToken capability tokens and callpack artifacts for voice boundaries.',
    href: '/docs/voice_mode',
  },
  {
    title: 'Context Evidence',
    description: 'Deterministic proof of what context the model was working from. Privacy-aware envelopes with fail-closed enforcement when evidence is missing.',
    href: '/docs/contracts/contextspec_v1',
  },
];

const faqs = [
  {
    question: 'What problem does Gait solve for long-running agent work?',
    answer:
      'Multi-step and multi-hour agent jobs fail mid-flight, losing state and provenance. Gait dispatches durable jobs with checkpointed state, pause/resume/cancel, and deterministic stop reasons so work survives failures and stays auditable.',
  },
  {
    question: 'What does Gait do that logs do not?',
    answer:
      'Gait produces signed packs and traces with deterministic verification, so incidents are portable, independently verifiable evidence rather than best-effort log interpretation.',
  },
  {
    question: 'Does Gait require a hosted service?',
    answer:
      'No. Core workflows are offline-first and run locally: capture, verify, diff, policy evaluation, and regressions can run without a network dependency.',
  },
  {
    question: 'How does Gait handle prompt-injection style risk?',
    answer:
      'Gate evaluates structured tool-call intent at execution time and blocks or requires approval based on policy. Non-allow outcomes do not execute side effects.',
  },
  {
    question: 'Can Gait gate voice agent actions?',
    answer:
      'Yes. Voice mode gates high-stakes spoken commitments (refunds, quotes, eligibility) before they are uttered. A signed SayToken capability token must be present for gated speech, and every call produces a signed callpack artifact.',
  },
  {
    question: 'What is context evidence?',
    answer:
      'Context evidence is a deterministic proof of what context material the model was working from at decision time. Gait captures privacy-aware context envelopes and enforces fail-closed policy when evidence is missing for high-risk actions.',
  },
  {
    question: 'How do I turn a failed agent run into a CI gate?',
    answer:
      'Run gait regress bootstrap --from <run_id> --junit output.xml. This converts the run into a permanent regression fixture. Exit 0 means pass, exit 5 means the same drift was detected. Wire the JUnit output into any CI system.',
  },
  {
    question: 'Can I replay an agent run without re-executing real API calls?',
    answer:
      'Yes. gait run replay uses recorded results as deterministic stubs so you can debug safely. gait pack diff then shows exactly what changed between two runs, including context drift classification.',
  },
];

const softwareApplicationJsonLd = {
  '@context': 'https://schema.org',
  '@type': 'SoftwareApplication',
  name: 'Gait',
  applicationCategory: 'DeveloperApplication',
  operatingSystem: 'Linux, macOS, Windows',
  description:
    'Offline-first runtime for production AI agents: durable jobs with checkpointed state, signed packs, voice agent gating, context evidence, deterministic regressions, and fail-closed policy gates at the tool boundary.',
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

      <div className="text-center py-12 lg:py-20">
        <h1 className="text-4xl lg:text-6xl font-bold text-white mb-6">
          Run Durable Agent Jobs.
          <span className="bg-gradient-to-r from-cyan-400 to-blue-500 bg-clip-text text-transparent"> Prove What Happened.</span>
        </h1>
        <p className="text-xl text-gray-400 max-w-3xl mx-auto mb-8">
          Gait is an offline-first runtime that dispatches durable agent jobs, captures state-changing tool calls,
          and emits signed packs you can verify, diff, and turn into deterministic CI regressions.
        </p>
        <div className="flex flex-col sm:flex-row gap-4 justify-center">
          <Link href="/docs/install" className="px-6 py-3 bg-cyan-500 hover:bg-cyan-400 text-gray-900 font-semibold rounded-lg transition-colors">
            Start Here
          </Link>
          <Link href="/docs/integration_checklist" className="px-6 py-3 bg-gray-800 hover:bg-gray-700 text-gray-100 font-semibold rounded-lg border border-gray-700 transition-colors">
            Integrate in 30-120 Minutes
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
              <td className="py-3 px-4 text-gray-300 font-medium">Long-running agent work</td>
              <td className="py-3 px-4 text-gray-500">fails mid-flight, lost state</td>
              <td className="py-3 px-4 text-gray-300">durable jobs with checkpoints + resume</td>
            </tr>
            <tr>
              <td className="py-3 px-4 text-gray-300 font-medium">Incident evidence</td>
              <td className="py-3 px-4 text-gray-500">logs + screenshots</td>
              <td className="py-3 px-4 text-gray-300">signed pack + ticket footer</td>
            </tr>
            <tr>
              <td className="py-3 px-4 text-gray-300 font-medium">Regression loop</td>
              <td className="py-3 px-4 text-gray-500">manual repro, often skipped</td>
              <td className="py-3 px-4 text-gray-300">deterministic fixture + CI gate</td>
            </tr>
            <tr>
              <td className="py-3 px-4 text-gray-300 font-medium">High-risk tool calls</td>
              <td className="py-3 px-4 text-gray-500">best-effort guardrails</td>
              <td className="py-3 px-4 text-gray-300">fail-closed policy + approvals</td>
            </tr>
            <tr>
              <td className="py-3 px-4 text-gray-300 font-medium">Voice agent commitments</td>
              <td className="py-3 px-4 text-gray-500">hope they say the right thing</td>
              <td className="py-3 px-4 text-gray-300">gated before speech + signed callpack</td>
            </tr>
            <tr>
              <td className="py-3 px-4 text-gray-300 font-medium">Audit posture</td>
              <td className="py-3 px-4 text-gray-500">incomplete reconstruction</td>
              <td className="py-3 px-4 text-gray-300">offline verifiable signed artifacts</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div className="mb-16">
        <h2 className="text-2xl font-bold text-white mb-6 text-center">Frequently Asked Questions</h2>
        <div className="grid md:grid-cols-2 gap-4">
          {faqs.map((entry) => (
            <div key={entry.question} className="rounded-lg border border-gray-700 bg-gray-900/40 p-5">
              <h3 className="text-base font-semibold text-gray-100 mb-2">{entry.question}</h3>
              <p className="text-sm text-gray-300">{entry.answer}</p>
            </div>
          ))}
        </div>
      </div>

      <div className="text-center py-12 border-t border-gray-800">
        <h2 className="text-2xl font-bold text-white mb-4">First pack in 60 seconds. Durable jobs, voice gating, and policy enforcement included.</h2>
        <p className="text-gray-400 mb-6">Install, create a signed artifact, and turn it into a permanent CI gate — all offline.</p>
        <Link href="/docs/install" className="inline-block px-6 py-3 bg-cyan-500 hover:bg-cyan-400 text-gray-900 font-semibold rounded-lg transition-colors">
          Open Install Guide
        </Link>
        <p className="text-sm text-gray-500 mt-5">
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
