package gate

import (
	"sort"
	"strings"

	schemacommon "github.com/Clyra-AI/gait/core/schema/v1/common"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
)

var (
	allowedRelationshipParentKinds = map[string]struct{}{
		"trace":    {},
		"run":      {},
		"session":  {},
		"intent":   {},
		"policy":   {},
		"agent":    {},
		"evidence": {},
	}
	allowedRelationshipEntityKinds = map[string]struct{}{
		"agent":      {},
		"tool":       {},
		"resource":   {},
		"policy":     {},
		"run":        {},
		"trace":      {},
		"delegation": {},
		"evidence":   {},
	}
	allowedRelationshipAgentRoles = map[string]struct{}{
		"requester": {},
		"delegator": {},
		"delegate":  {},
	}
	allowedRelationshipEdgeKinds = map[string]struct{}{
		"delegates_to": {},
		"calls":        {},
		"governed_by":  {},
		"targets":      {},
		"derived_from": {},
		"emits_evidence": {},
	}
)

func buildTraceRelationship(
	intent schemagate.IntentRequest,
	traceID string,
	policyID string,
	policyVersion string,
	policyDigest string,
	matchedRuleIDs []string,
) *schemacommon.RelationshipEnvelope {
	relationship := schemacommon.RelationshipEnvelope{}
	if parentKind, parentID := parentRefFromIntent(intent.Context); parentID != "" {
		relationship.ParentRef = &schemacommon.RelationshipNodeRef{Kind: parentKind, ID: parentID}
	}

	entityRefs := []schemacommon.RelationshipRef{}
	if traceID = strings.TrimSpace(traceID); traceID != "" {
		entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "trace", ID: traceID})
	}
	if toolName := strings.TrimSpace(intent.ToolName); toolName != "" {
		entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "tool", ID: toolName})
	}
	if identity := strings.TrimSpace(intent.Context.Identity); identity != "" {
		entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "agent", ID: identity})
	}
	if policyDigest = strings.ToLower(strings.TrimSpace(policyDigest)); policyDigest != "" {
		entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "policy", ID: policyDigest})
	}
	relationship.EntityRefs = normalizeRelationshipRefs(entityRefs)
	relationship.RelatedEntityIDs = relationshipRefIDs(relationship.EntityRefs)

	policyID = strings.TrimSpace(policyID)
	policyVersion = strings.TrimSpace(policyVersion)
	matchedRuleIDs = uniqueSorted(matchedRuleIDs)
	if policyID != "" || policyVersion != "" || policyDigest != "" || len(matchedRuleIDs) > 0 {
		relationship.PolicyRef = &schemacommon.PolicyRef{
			PolicyID:       policyID,
			PolicyVersion:  policyVersion,
			PolicyDigest:   policyDigest,
			MatchedRuleIDs: matchedRuleIDs,
		}
	}

	agentChain := []schemacommon.AgentLink{}
	if intent.Delegation != nil {
		if requester := strings.TrimSpace(intent.Delegation.RequesterIdentity); requester != "" {
			agentChain = append(agentChain, schemacommon.AgentLink{Identity: requester, Role: "requester"})
		}
		for _, link := range intent.Delegation.Chain {
			if delegator := strings.TrimSpace(link.DelegatorIdentity); delegator != "" {
				agentChain = append(agentChain, schemacommon.AgentLink{Identity: delegator, Role: "delegator"})
			}
			if delegate := strings.TrimSpace(link.DelegateIdentity); delegate != "" {
				agentChain = append(agentChain, schemacommon.AgentLink{Identity: delegate, Role: "delegate"})
			}
		}
	} else if identity := strings.TrimSpace(intent.Context.Identity); identity != "" {
		agentChain = append(agentChain, schemacommon.AgentLink{Identity: identity, Role: "requester"})
	}
	relationship.AgentChain = normalizeAgentChain(agentChain)

	edges := []schemacommon.RelationshipEdge{}
	if actor := strings.TrimSpace(intent.Context.Identity); actor != "" && strings.TrimSpace(intent.ToolName) != "" {
		edges = append(edges, schemacommon.RelationshipEdge{
			Kind: "calls",
			From: schemacommon.RelationshipNodeRef{Kind: "agent", ID: actor},
			To:   schemacommon.RelationshipNodeRef{Kind: "tool", ID: strings.TrimSpace(intent.ToolName)},
		})
	}
	if strings.TrimSpace(intent.ToolName) != "" && policyDigest != "" {
		edges = append(edges, schemacommon.RelationshipEdge{
			Kind: "governed_by",
			From: schemacommon.RelationshipNodeRef{Kind: "tool", ID: strings.TrimSpace(intent.ToolName)},
			To:   schemacommon.RelationshipNodeRef{Kind: "policy", ID: policyDigest},
		})
	}
	if intent.Delegation != nil {
		for _, link := range intent.Delegation.Chain {
			delegator := strings.TrimSpace(link.DelegatorIdentity)
			delegate := strings.TrimSpace(link.DelegateIdentity)
			if delegator == "" || delegate == "" {
				continue
			}
			edges = append(edges, schemacommon.RelationshipEdge{
				Kind: "delegates_to",
				From: schemacommon.RelationshipNodeRef{Kind: "agent", ID: delegator},
				To:   schemacommon.RelationshipNodeRef{Kind: "agent", ID: delegate},
			})
		}
	}
	relationship.Edges = normalizeRelationshipEdges(edges)
	return normalizeRelationshipEnvelope(&relationship)
}

func buildApprovalAuditRelationship(traceID, toolName, policyDigest string, approvers []string) *schemacommon.RelationshipEnvelope {
	traceID = strings.TrimSpace(traceID)
	relationship := schemacommon.RelationshipEnvelope{
		ParentRef: &schemacommon.RelationshipNodeRef{
			Kind: "trace",
			ID:   traceID,
		},
		ParentRecordID: traceID,
	}
	if relationship.ParentRef.ID == "" {
		relationship.ParentRef = nil
	}
	toolName = strings.TrimSpace(toolName)
	policyDigest = strings.ToLower(strings.TrimSpace(policyDigest))

	entityRefs := []schemacommon.RelationshipRef{}
	if traceID != "" {
		entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "trace", ID: traceID})
	}
	if toolName != "" {
		entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "tool", ID: toolName})
	}
	if policyDigest != "" {
		entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "policy", ID: policyDigest})
		relationship.PolicyRef = &schemacommon.PolicyRef{PolicyDigest: policyDigest}
	}
	approvers = uniqueSorted(approvers)
	for _, approver := range approvers {
		if approver = strings.TrimSpace(approver); approver != "" {
			entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "agent", ID: approver})
		}
	}
	relationship.EntityRefs = normalizeRelationshipRefs(entityRefs)
	relationship.RelatedEntityIDs = relationshipRefIDs(relationship.EntityRefs)

	edges := []schemacommon.RelationshipEdge{}
	if toolName != "" && policyDigest != "" {
		edges = append(edges, schemacommon.RelationshipEdge{
			Kind: "governed_by",
			From: schemacommon.RelationshipNodeRef{Kind: "tool", ID: toolName},
			To:   schemacommon.RelationshipNodeRef{Kind: "policy", ID: policyDigest},
		})
	}
	relationship.Edges = normalizeRelationshipEdges(edges)
	return normalizeRelationshipEnvelope(&relationship)
}

func buildDelegationAuditRelationship(traceID, toolName, policyDigest string, entries []schemagate.DelegationAuditEntry) *schemacommon.RelationshipEnvelope {
	traceID = strings.TrimSpace(traceID)
	relationship := schemacommon.RelationshipEnvelope{
		ParentRef: &schemacommon.RelationshipNodeRef{
			Kind: "trace",
			ID:   traceID,
		},
		ParentRecordID: traceID,
	}
	if relationship.ParentRef.ID == "" {
		relationship.ParentRef = nil
	}
	toolName = strings.TrimSpace(toolName)
	policyDigest = strings.ToLower(strings.TrimSpace(policyDigest))

	entityRefs := []schemacommon.RelationshipRef{}
	if traceID != "" {
		entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "trace", ID: traceID})
	}
	if toolName != "" {
		entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "tool", ID: toolName})
	}
	if policyDigest != "" {
		entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "policy", ID: policyDigest})
		relationship.PolicyRef = &schemacommon.PolicyRef{PolicyDigest: policyDigest}
	}

	agentChain := []schemacommon.AgentLink{}
	edges := []schemacommon.RelationshipEdge{}
	for _, entry := range entries {
		delegator := strings.TrimSpace(entry.DelegatorIdentity)
		delegate := strings.TrimSpace(entry.DelegateIdentity)
		if delegator != "" {
			entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "agent", ID: delegator})
			agentChain = append(agentChain, schemacommon.AgentLink{Identity: delegator, Role: "delegator"})
		}
		if delegate != "" {
			entityRefs = append(entityRefs, schemacommon.RelationshipRef{Kind: "agent", ID: delegate})
			agentChain = append(agentChain, schemacommon.AgentLink{Identity: delegate, Role: "delegate"})
		}
		if delegator != "" && delegate != "" {
			edges = append(edges, schemacommon.RelationshipEdge{
				Kind: "delegates_to",
				From: schemacommon.RelationshipNodeRef{Kind: "agent", ID: delegator},
				To:   schemacommon.RelationshipNodeRef{Kind: "agent", ID: delegate},
			})
		}
	}
	if toolName != "" && policyDigest != "" {
		edges = append(edges, schemacommon.RelationshipEdge{
			Kind: "governed_by",
			From: schemacommon.RelationshipNodeRef{Kind: "tool", ID: toolName},
			To:   schemacommon.RelationshipNodeRef{Kind: "policy", ID: policyDigest},
		})
	}

	relationship.EntityRefs = normalizeRelationshipRefs(entityRefs)
	relationship.RelatedEntityIDs = relationshipRefIDs(relationship.EntityRefs)
	relationship.AgentChain = normalizeAgentChain(agentChain)
	relationship.Edges = normalizeRelationshipEdges(edges)
	return normalizeRelationshipEnvelope(&relationship)
}

func normalizeRelationshipEnvelope(envelope *schemacommon.RelationshipEnvelope) *schemacommon.RelationshipEnvelope {
	if envelope == nil {
		return nil
	}
	normalized := *envelope
	normalized.ParentRecordID = strings.TrimSpace(normalized.ParentRecordID)
	normalized.RelatedRecordIDs = uniqueSorted(normalized.RelatedRecordIDs)
	normalized.RelatedEntityIDs = uniqueSorted(normalized.RelatedEntityIDs)
	normalized.ParentRef = normalizeParentRef(normalized.ParentRef)
	normalized.EntityRefs = normalizeRelationshipRefs(normalized.EntityRefs)
	normalized.AgentChain = normalizeAgentChain(normalized.AgentChain)
	normalized.Edges = normalizeRelationshipEdges(normalized.Edges)
	normalized.AgentLineage = normalizeAgentLineage(normalized.AgentLineage)
	if normalized.PolicyRef != nil {
		normalized.PolicyRef.PolicyID = strings.TrimSpace(normalized.PolicyRef.PolicyID)
		normalized.PolicyRef.PolicyVersion = strings.TrimSpace(normalized.PolicyRef.PolicyVersion)
		normalized.PolicyRef.PolicyDigest = strings.ToLower(strings.TrimSpace(normalized.PolicyRef.PolicyDigest))
		normalized.PolicyRef.MatchedRuleIDs = uniqueSorted(normalized.PolicyRef.MatchedRuleIDs)
		if normalized.PolicyRef.PolicyID == "" &&
			normalized.PolicyRef.PolicyVersion == "" &&
			normalized.PolicyRef.PolicyDigest == "" &&
			len(normalized.PolicyRef.MatchedRuleIDs) == 0 {
			normalized.PolicyRef = nil
		}
	}
	if len(normalized.RelatedEntityIDs) == 0 {
		normalized.RelatedEntityIDs = relationshipRefIDs(normalized.EntityRefs)
	}
	if isRelationshipEnvelopeEmpty(normalized) {
		return nil
	}
	return &normalized
}

func normalizeParentRef(ref *schemacommon.RelationshipNodeRef) *schemacommon.RelationshipNodeRef {
	if ref == nil {
		return nil
	}
	kind := strings.ToLower(strings.TrimSpace(ref.Kind))
	id := strings.TrimSpace(ref.ID)
	if kind == "" || id == "" {
		return nil
	}
	if _, ok := allowedRelationshipParentKinds[kind]; !ok {
		return nil
	}
	return &schemacommon.RelationshipNodeRef{Kind: kind, ID: id}
}

func normalizeAgentLineage(values []schemacommon.AgentLineage) []schemacommon.AgentLineage {
	if len(values) == 0 {
		return nil
	}
	out := make([]schemacommon.AgentLineage, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		agentID := strings.TrimSpace(value.AgentID)
		delegatedBy := strings.TrimSpace(value.DelegatedBy)
		delegationRecordID := strings.TrimSpace(value.DelegationRecordID)
		if agentID == "" {
			continue
		}
		key := agentID + "\x00" + delegatedBy + "\x00" + delegationRecordID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, schemacommon.AgentLineage{
			AgentID:            agentID,
			DelegatedBy:        delegatedBy,
			DelegationRecordID: delegationRecordID,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AgentID != out[j].AgentID {
			return out[i].AgentID < out[j].AgentID
		}
		if out[i].DelegatedBy != out[j].DelegatedBy {
			return out[i].DelegatedBy < out[j].DelegatedBy
		}
		return out[i].DelegationRecordID < out[j].DelegationRecordID
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

func relationshipRefIDs(refs []schemacommon.RelationshipRef) []string {
	if len(refs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if id := strings.TrimSpace(ref.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return uniqueSorted(ids)
}

func parentRefFromIntent(context schemagate.IntentContext) (string, string) {
	if sessionID := strings.TrimSpace(context.SessionID); sessionID != "" {
		return "session", sessionID
	}
	return "", ""
}

func normalizeRelationshipRefs(refs []schemacommon.RelationshipRef) []schemacommon.RelationshipRef {
	if len(refs) == 0 {
		return nil
	}
	normalized := make([]schemacommon.RelationshipRef, 0, len(refs))
	seen := map[string]struct{}{}
	for _, ref := range refs {
		kind := strings.ToLower(strings.TrimSpace(ref.Kind))
		id := strings.TrimSpace(ref.ID)
		if kind == "" || id == "" {
			continue
		}
		if _, ok := allowedRelationshipEntityKinds[kind]; !ok {
			continue
		}
		key := kind + "\x00" + id
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, schemacommon.RelationshipRef{Kind: kind, ID: id})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Kind != normalized[j].Kind {
			return normalized[i].Kind < normalized[j].Kind
		}
		return normalized[i].ID < normalized[j].ID
	})
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeAgentChain(chain []schemacommon.AgentLink) []schemacommon.AgentLink {
	if len(chain) == 0 {
		return nil
	}
	normalized := make([]schemacommon.AgentLink, 0, len(chain))
	seen := map[string]struct{}{}
	for _, link := range chain {
		role := strings.ToLower(strings.TrimSpace(link.Role))
		identity := strings.TrimSpace(link.Identity)
		if role == "" || identity == "" {
			continue
		}
		if _, ok := allowedRelationshipAgentRoles[role]; !ok {
			continue
		}
		key := role + "\x00" + identity
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, schemacommon.AgentLink{Role: role, Identity: identity})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Role != normalized[j].Role {
			return normalized[i].Role < normalized[j].Role
		}
		return normalized[i].Identity < normalized[j].Identity
	})
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeRelationshipEdges(edges []schemacommon.RelationshipEdge) []schemacommon.RelationshipEdge {
	if len(edges) == 0 {
		return nil
	}
	normalized := make([]schemacommon.RelationshipEdge, 0, len(edges))
	seen := map[string]struct{}{}
	for _, edge := range edges {
		kind := strings.ToLower(strings.TrimSpace(edge.Kind))
		fromKind := strings.ToLower(strings.TrimSpace(edge.From.Kind))
		fromID := strings.TrimSpace(edge.From.ID)
		toKind := strings.ToLower(strings.TrimSpace(edge.To.Kind))
		toID := strings.TrimSpace(edge.To.ID)
		if kind == "" || fromKind == "" || fromID == "" || toKind == "" || toID == "" {
			continue
		}
		if _, ok := allowedRelationshipEdgeKinds[kind]; !ok {
			continue
		}
		key := kind + "\x00" + fromKind + "\x00" + fromID + "\x00" + toKind + "\x00" + toID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, schemacommon.RelationshipEdge{
			Kind: kind,
			From: schemacommon.RelationshipNodeRef{Kind: fromKind, ID: fromID},
			To:   schemacommon.RelationshipNodeRef{Kind: toKind, ID: toID},
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Kind != normalized[j].Kind {
			return normalized[i].Kind < normalized[j].Kind
		}
		if normalized[i].From.Kind != normalized[j].From.Kind {
			return normalized[i].From.Kind < normalized[j].From.Kind
		}
		if normalized[i].From.ID != normalized[j].From.ID {
			return normalized[i].From.ID < normalized[j].From.ID
		}
		if normalized[i].To.Kind != normalized[j].To.Kind {
			return normalized[i].To.Kind < normalized[j].To.Kind
		}
		return normalized[i].To.ID < normalized[j].To.ID
	})
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func isRelationshipEnvelopeEmpty(envelope schemacommon.RelationshipEnvelope) bool {
	return envelope.ParentRef == nil &&
		len(envelope.EntityRefs) == 0 &&
		envelope.PolicyRef == nil &&
		len(envelope.AgentChain) == 0 &&
		len(envelope.Edges) == 0 &&
		strings.TrimSpace(envelope.ParentRecordID) == "" &&
		len(envelope.RelatedRecordIDs) == 0 &&
		len(envelope.RelatedEntityIDs) == 0 &&
		len(envelope.AgentLineage) == 0
}
