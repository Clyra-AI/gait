import Link from 'next/link';
import type { Metadata } from 'next';
import { canonicalUrl } from '@/lib/site';

export const metadata: Metadata = {
  title: 'Gait Documentation',
  description: 'Start-to-production documentation ladder for Gait OSS.',
  alternates: { canonical: canonicalUrl('/docs/') },
};

const tracks = [
  {
    title: 'Track 1: First Win (5 Minutes)',
    steps: [
      { label: 'Install', href: '/docs/install' },
      { label: 'Adopt In One PR', href: '/docs/adopt_in_one_pr' },
      { label: 'Mental Model', href: '/docs/concepts/mental_model' },
      { label: 'Flows', href: '/docs/flows' },
      { label: 'Simple Scenario', href: '/docs/scenarios/simple_agent_tool_boundary' },
      { label: 'Demo Output Legend', href: '/docs/demo_output_legend' },
    ],
  },
  {
    title: 'Track 2: Integration (30-120 Minutes)',
    steps: [
      { label: 'Integration Checklist', href: '/docs/integration_checklist' },
      { label: 'Integration Boundary Guide', href: '/docs/agent_integration_boundary' },
      { label: 'MCP Capability Matrix', href: '/docs/mcp_capability_matrix' },
      { label: 'Cloud Runtime Patterns', href: '/docs/deployment/cloud_runtime_patterns' },
      { label: 'Policy Authoring', href: '/docs/policy_authoring' },
      { label: 'Python SDK Contract', href: '/docs/sdk/python' },
      { label: 'Project Defaults', href: '/docs/project_defaults' },
      { label: 'Durable Jobs', href: '/docs/durable_jobs' },
      { label: 'CI Regress Kit', href: '/docs/ci_regress_kit' },
    ],
  },
  {
    title: 'Track 3: Production Posture',
    steps: [
      { label: 'Policy Rollout', href: '/docs/policy_rollout' },
      { label: 'Approval Runbook', href: '/docs/approval_runbook' },
      { label: 'Primitive Contract', href: '/docs/contracts/primitive_contract' },
      { label: 'Pack Producer Kit', href: '/docs/contracts/pack_producer_kit' },
      { label: 'Compatibility Matrix', href: '/docs/contracts/compatibility_matrix' },
      { label: 'Failure Taxonomy + Exit Codes', href: '/docs/failure_taxonomy_exit_codes' },
      { label: 'Threat Model', href: '/docs/threat_model' },
      { label: 'Runtime SLO', href: '/docs/slo/runtime_slo' },
    ],
  },
  {
    title: 'Track 4: Governance and Enterprise Fit',
    steps: [
      { label: 'Zero Trust Stack', href: '/docs/zero_trust_stack' },
      { label: 'External Tool Registry Policy', href: '/docs/external_tool_registry_policy' },
      { label: 'SIEM Ingestion Recipes', href: '/docs/siem_ingestion_recipes' },
      { label: 'Positioning', href: '/docs/positioning' },
      { label: 'Evidence Templates', href: '/docs/evidence_templates' },
    ],
  },
  {
    title: 'Track 5: Ecosystem Distribution',
    steps: [
      { label: 'Launch Kit', href: '/docs/launch/readme' },
      { label: 'Secure Deploy OpenClaw', href: '/docs/launch/secure_deployment_openclaw' },
      { label: 'Secure Deploy Gas Town', href: '/docs/launch/secure_deployment_gastown' },
    ],
  },
];

export default function DocsHomePage() {
  return (
    <div className="not-prose">
      <h1 className="text-3xl lg:text-4xl font-bold text-white mb-4">Documentation</h1>
      <p className="text-gray-400 mb-10 max-w-3xl">
        Use this ladder to go from first runpack to fail-closed production rollout without guessing.
      </p>

      <div className="grid gap-6">
        {tracks.map((track) => (
          <section key={track.title} className="rounded-xl border border-gray-700 bg-gray-900/30 p-6">
            <h2 className="text-xl font-semibold text-white mb-4">{track.title}</h2>
            <div className="flex flex-wrap gap-3">
              {track.steps.map((step) => (
                <Link
                  key={step.href}
                  href={step.href}
                  className="inline-flex items-center rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-200 hover:bg-gray-800/70"
                >
                  {step.label}
                </Link>
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}
