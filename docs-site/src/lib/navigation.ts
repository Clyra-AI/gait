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
    ],
  },
  {
    title: 'Operate',
    href: '/docs/integration_checklist',
    children: [
      { title: 'Integration Checklist', href: '/docs/integration_checklist' },
      { title: 'Policy Rollout', href: '/docs/policy_rollout' },
      { title: 'Approval Runbook', href: '/docs/approval_runbook' },
      { title: 'Project Defaults', href: '/docs/project_defaults' },
      { title: 'CI Regress Kit', href: '/docs/ci_regress_kit' },
      { title: 'UAT Plan', href: '/docs/uat_functional_plan' },
    ],
  },
  {
    title: 'Contracts',
    href: '/docs/contracts/primitive_contract',
    children: [
      { title: 'Primitive Contract', href: '/docs/contracts/primitive_contract' },
      { title: 'Endpoint Action Model', href: '/docs/contracts/endpoint_action_model' },
      { title: 'Skill Provenance', href: '/docs/contracts/skill_provenance' },
      { title: 'Runtime SLO', href: '/docs/slo/runtime_slo' },
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
