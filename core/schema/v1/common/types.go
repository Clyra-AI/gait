package common

type RelationshipEnvelope struct {
	ParentRef       *RelationshipNodeRef `json:"parent_ref,omitempty"`
	EntityRefs      []RelationshipRef    `json:"entity_refs,omitempty"`
	PolicyRef       *PolicyRef           `json:"policy_ref,omitempty"`
	AgentChain      []AgentLink          `json:"agent_chain,omitempty"`
	Edges           []RelationshipEdge   `json:"edges,omitempty"`
	ParentRecordID  string               `json:"parent_record_id,omitempty"`
	RelatedRecordIDs []string            `json:"related_record_ids,omitempty"`
	RelatedEntityIDs []string            `json:"related_entity_ids,omitempty"`
	AgentLineage    []AgentLineage       `json:"agent_lineage,omitempty"`
}

type RelationshipNodeRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type RelationshipRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type PolicyRef struct {
	PolicyID       string   `json:"policy_id,omitempty"`
	PolicyVersion  string   `json:"policy_version,omitempty"`
	PolicyDigest   string   `json:"policy_digest,omitempty"`
	MatchedRuleIDs []string `json:"matched_rule_ids,omitempty"`
}

type AgentLink struct {
	Identity string `json:"identity"`
	Role     string `json:"role"`
}

type RelationshipEdge struct {
	Kind string              `json:"kind"`
	From RelationshipNodeRef `json:"from"`
	To   RelationshipNodeRef `json:"to"`
}

type AgentLineage struct {
	AgentID            string `json:"agent_id"`
	DelegatedBy        string `json:"delegated_by,omitempty"`
	DelegationRecordID string `json:"delegation_record_id,omitempty"`
}
