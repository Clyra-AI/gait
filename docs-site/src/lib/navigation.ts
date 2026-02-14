export interface NavItem {
  title: string;
  href: string;
  children?: NavItem[];
}

export const navigation: NavItem[] = [
  {
    title: 'Start Here',
    href: '/docs',
    children: [
      { title: 'Install', href: '/docs/install' },
      { title: 'Mental Model', href: '/docs/concepts/mental_model' },
      { title: 'Architecture', href: '/docs/architecture' },
      { title: 'Flows', href: '/docs/flows' },
      { title: 'Simple Scenario', href: '/docs/scenarios/simple_agent_tool_boundary' },
      { title: 'Demo Output Legend', href: '/docs/demo_output_legend' },
    ],
  },
  {
    title: 'Operate',
    href: '/docs/integration_checklist',
    children: [
      { title: 'Integration Checklist', href: '/docs/integration_checklist' },
      { title: 'Integration Boundary Guide', href: '/docs/agent_integration_boundary' },
      { title: 'MCP Capability Matrix', href: '/docs/mcp_capability_matrix' },
      { title: 'Cloud Runtime Patterns', href: '/docs/deployment/cloud_runtime_patterns' },
      { title: 'Policy Authoring', href: '/docs/policy_authoring' },
      { title: 'Policy Rollout', href: '/docs/policy_rollout' },
      { title: 'Approval Runbook', href: '/docs/approval_runbook' },
      { title: 'Project Defaults', href: '/docs/project_defaults' },
      { title: 'CI Regress Kit', href: '/docs/ci_regress_kit' },
      { title: 'UAT Plan', href: '/docs/uat_functional_plan' },
    ],
  },
  {
    title: 'Governance',
    href: '/docs/zero_trust_stack',
    children: [
      { title: 'Zero Trust Stack', href: '/docs/zero_trust_stack' },
      { title: 'External Tool Registry Policy', href: '/docs/external_tool_registry_policy' },
      { title: 'SIEM Ingestion Recipes', href: '/docs/siem_ingestion_recipes' },
      { title: 'Positioning', href: '/docs/positioning' },
      { title: 'Evidence Templates', href: '/docs/evidence_templates' },
    ],
  },
  {
    title: 'Launch',
    href: '/docs/launch/readme',
    children: [
      { title: 'Launch Kit', href: '/docs/launch/readme' },
      { title: 'Secure Deploy OpenClaw', href: '/docs/launch/secure_deployment_openclaw' },
      { title: 'Secure Deploy Gas Town', href: '/docs/launch/secure_deployment_gastown' },
    ],
  },
  {
    title: 'Contracts',
    href: '/docs/contracts/primitive_contract',
    children: [
      { title: 'Primitive Contract', href: '/docs/contracts/primitive_contract' },
      { title: 'Artifact Graph Contract', href: '/docs/contracts/artifact_graph' },
      { title: 'Endpoint Action Model', href: '/docs/contracts/endpoint_action_model' },
      { title: 'Skill Provenance', href: '/docs/contracts/skill_provenance' },
      { title: 'Runtime SLO', href: '/docs/slo/runtime_slo' },
    ],
  },
  {
    title: 'Blog',
    href: '/docs/blog/openclaw_24h_boundary_enforcement',
    children: [
      { title: '2,880 Tool Calls Gate-Checked', href: '/docs/blog/openclaw_24h_boundary_enforcement' },
    ],
  },
  {
    title: 'Ecosystem',
    href: '/docs/ecosystem/awesome',
    children: [
      { title: 'Community Index', href: '/docs/ecosystem/awesome' },
      { title: 'Contribute', href: '/docs/ecosystem/contribute' },
      { title: 'Homebrew', href: '/docs/homebrew' },
      { title: 'Packaging', href: '/docs/packaging' },
      { title: 'Security Policy', href: '/docs/security' },
      { title: 'Contributing Guide', href: '/docs/contributing' },
    ],
  },
];
