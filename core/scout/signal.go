package scout

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Clyra-AI/gait/core/jcs"
	"github.com/Clyra-AI/gait/core/runpack"
	schemagate "github.com/Clyra-AI/gait/core/schema/v1/gate"
	schemaregress "github.com/Clyra-AI/gait/core/schema/v1/regress"
	schemascout "github.com/Clyra-AI/gait/core/schema/v1/scout"
)

const (
	runFingerprintSchemaID  = "gait.scout.run_fingerprint"
	runFingerprintSchemaV1  = "1.0.0"
	signalReportSchemaID    = "gait.scout.signal_report"
	signalReportSchemaV1    = "1.0.0"
	topIssueLimit           = 10
	defaultSignalTimeYear   = 1980
	signalSeverityLow       = "low"
	signalSeverityMedium    = "medium"
	signalSeverityHigh      = "high"
	signalSeverityCritical  = "critical"
	driverPolicyChange      = "policy_change"
	driverToolResultChange  = "tool_result_shape_change"
	driverReferenceSet      = "reference_set_change"
	driverConfiguration     = "configuration_change"
	suggestionPolicyFixture = "policy_fixture"
	suggestionRegress       = "regress_fixture"
	suggestionConfig        = "configuration"
	regressStatusPass       = "pass"
)

var runIDPattern = regexp.MustCompile(`run_[A-Za-z0-9_-]+`)

type SignalInput struct {
	RunpackPaths []string
	TracePaths   []string
	RegressPaths []string
}

type SignalOptions struct {
	ProducerVersion string
	Now             time.Time
}

type signalObservation struct {
	runID               string
	fingerprint         string
	actionSequence      []string
	toolClasses         []string
	targetSystems       []string
	reasonCodeVector    []string
	refReceiptDigests   []string
	sourceRunpack       string
	traceCount          int
	regressEvidenceSeen bool
	maxPrivilegeScore   int
	targetSensitivity   int
	policyPosture       int
	baseSeverityScore   int
}

func BuildSignalReport(input SignalInput, opts SignalOptions) (schemascout.SignalReport, error) {
	runpackPaths := uniqueSorted(input.RunpackPaths)
	if len(runpackPaths) == 0 {
		return schemascout.SignalReport{}, fmt.Errorf("at least one runpack path is required")
	}

	traceReasonsByRun, traceCountByRun, err := loadTraceSignals(uniqueSorted(input.TracePaths))
	if err != nil {
		return schemascout.SignalReport{}, err
	}
	regressReasonsByRun, err := loadRegressSignals(uniqueSorted(input.RegressPaths))
	if err != nil {
		return schemascout.SignalReport{}, err
	}

	observations := make([]signalObservation, 0, len(runpackPaths))
	for _, runpackPath := range runpackPaths {
		observation, buildErr := buildObservation(runpackPath, traceReasonsByRun, traceCountByRun, regressReasonsByRun)
		if buildErr != nil {
			return schemascout.SignalReport{}, buildErr
		}
		observations = append(observations, observation)
	}

	sort.Slice(observations, func(i, j int) bool {
		if observations[i].runID != observations[j].runID {
			return observations[i].runID < observations[j].runID
		}
		return observations[i].sourceRunpack < observations[j].sourceRunpack
	})

	fingerprints := make([]schemascout.RunFingerprint, 0, len(observations))
	for _, observation := range observations {
		fingerprints = append(fingerprints, schemascout.RunFingerprint{
			SchemaID:            runFingerprintSchemaID,
			SchemaVersion:       runFingerprintSchemaV1,
			CreatedAt:           normalizeSignalNow(opts.Now),
			ProducerVersion:     normalizeProducerVersion(opts.ProducerVersion),
			RunID:               observation.runID,
			Fingerprint:         observation.fingerprint,
			ActionSequence:      append([]string{}, observation.actionSequence...),
			ToolClasses:         append([]string{}, observation.toolClasses...),
			TargetSystems:       append([]string{}, observation.targetSystems...),
			ReasonCodeVector:    append([]string{}, observation.reasonCodeVector...),
			RefReceiptDigests:   append([]string{}, observation.refReceiptDigests...),
			SourceRunpack:       observation.sourceRunpack,
			TraceCount:          observation.traceCount,
			RegressEvidenceSeen: observation.regressEvidenceSeen,
		})
	}

	families := buildSignalFamilies(observations)
	topIssues := make([]schemascout.SignalIssue, 0, len(families))
	for index, family := range families {
		if index >= topIssueLimit {
			break
		}
		topIssues = append(topIssues, schemascout.SignalIssue{
			Rank:             index + 1,
			FamilyID:         family.FamilyID,
			Fingerprint:      family.Fingerprint,
			Count:            family.Count,
			CanonicalRunID:   family.CanonicalRunID,
			TopFailureReason: family.TopFailureReason,
			Drivers:          append([]string{}, family.Drivers...),
			SeverityScore:    family.SeverityScore,
			SeverityLevel:    family.SeverityLevel,
			Suggestions:      append([]schemascout.SignalFixSuggestion{}, family.Suggestions...),
		})
	}

	return schemascout.SignalReport{
		SchemaID:        signalReportSchemaID,
		SchemaVersion:   signalReportSchemaV1,
		CreatedAt:       normalizeSignalNow(opts.Now),
		ProducerVersion: normalizeProducerVersion(opts.ProducerVersion),
		RunCount:        len(observations),
		FamilyCount:     len(families),
		Fingerprints:    fingerprints,
		Families:        families,
		TopIssues:       topIssues,
	}, nil
}

func buildObservation(
	runpackPath string,
	traceReasonsByRun map[string][]string,
	traceCountByRun map[string]int,
	regressReasonsByRun map[string][]string,
) (signalObservation, error) {
	pack, err := runpack.ReadRunpack(runpackPath)
	if err != nil {
		return signalObservation{}, fmt.Errorf("read runpack %s: %w", runpackPath, err)
	}
	runID := strings.TrimSpace(pack.Run.RunID)
	if runID == "" {
		return signalObservation{}, fmt.Errorf("runpack %s missing run_id", runpackPath)
	}

	actionSequence := make([]string, 0, len(pack.Intents))
	toolClasses := make([]string, 0, len(pack.Intents))
	maxPrivilege := 1
	for _, intent := range pack.Intents {
		toolName := strings.TrimSpace(intent.ToolName)
		if toolName == "" {
			continue
		}
		actionSequence = append(actionSequence, toolName)
		class := classifyToolClass(toolName)
		toolClasses = append(toolClasses, class)
		if score := toolClassScore(class); score > maxPrivilege {
			maxPrivilege = score
		}
	}
	toolClasses = uniqueSorted(toolClasses)

	targetSystems := make([]string, 0, len(pack.Refs.Receipts))
	refReceiptDigests := make([]string, 0, len(pack.Refs.Receipts))
	for _, receipt := range pack.Refs.Receipts {
		targetSystems = append(targetSystems, normalizeTargetSystem(receipt.SourceType, receipt.SourceLocator))
		digest := strings.ToLower(strings.TrimSpace(receipt.ContentDigest))
		if len(digest) == 64 {
			refReceiptDigests = append(refReceiptDigests, digest)
		}
	}
	targetSystems = uniqueSorted(targetSystems)
	refReceiptDigests = uniqueSorted(refReceiptDigests)

	reasonCodes := make([]string, 0, len(pack.Results))
	for _, result := range pack.Results {
		status := normalizeIdentifier(strings.ToLower(strings.TrimSpace(result.Status)))
		if status == "" || status == "ok" || status == "success" {
			continue
		}
		reasonCodes = append(reasonCodes, "result_status_"+status)
	}
	reasonCodes = append(reasonCodes, traceReasonsByRun[runID]...)
	reasonCodes = append(reasonCodes, regressReasonsByRun[runID]...)
	reasonCodes = uniqueSorted(reasonCodes)

	targetSensitivity := targetSensitivityScore(targetSystems)
	policyPosture := policyPostureScore(reasonCodes)
	baseSeverity := maxPrivilege*30 + targetSensitivity*15 + policyPosture*10 + minInt(len(reasonCodes), 9)

	fingerprint, err := computeRunFingerprint(actionSequence, toolClasses, targetSystems, reasonCodes, refReceiptDigests)
	if err != nil {
		return signalObservation{}, fmt.Errorf("compute run fingerprint for %s: %w", runID, err)
	}

	return signalObservation{
		runID:               runID,
		fingerprint:         fingerprint,
		actionSequence:      actionSequence,
		toolClasses:         toolClasses,
		targetSystems:       targetSystems,
		reasonCodeVector:    reasonCodes,
		refReceiptDigests:   refReceiptDigests,
		sourceRunpack:       runpackPath,
		traceCount:          traceCountByRun[runID],
		regressEvidenceSeen: len(regressReasonsByRun[runID]) > 0,
		maxPrivilegeScore:   maxPrivilege,
		targetSensitivity:   targetSensitivity,
		policyPosture:       policyPosture,
		baseSeverityScore:   baseSeverity,
	}, nil
}

func buildSignalFamilies(observations []signalObservation) []schemascout.SignalFamily {
	familyBuckets := map[string][]signalObservation{}
	for _, observation := range observations {
		familyBuckets[observation.fingerprint] = append(familyBuckets[observation.fingerprint], observation)
	}

	families := make([]schemascout.SignalFamily, 0, len(familyBuckets))
	for fingerprint, members := range familyBuckets {
		runIDs := make([]string, 0, len(members))
		artifactPointers := make([]string, 0, len(members))
		allReasons := make([]string, 0)
		maxSeverity := 0
		for _, member := range members {
			runIDs = append(runIDs, member.runID)
			artifactPointers = append(artifactPointers, member.sourceRunpack)
			allReasons = append(allReasons, member.reasonCodeVector...)
			if member.baseSeverityScore > maxSeverity {
				maxSeverity = member.baseSeverityScore
			}
		}
		runIDs = uniqueSorted(runIDs)
		artifactPointers = uniqueSorted(artifactPointers)
		allReasons = uniqueSorted(allReasons)
		drivers := inferDriverCategories(allReasons)
		suggestions := suggestionsForDrivers(drivers)
		severityScore := maxSeverity + minInt(len(members), 10)

		familyID := "fam_" + fingerprint[:12]
		families = append(families, schemascout.SignalFamily{
			FamilyID:         familyID,
			Fingerprint:      fingerprint,
			Count:            len(members),
			RunIDs:           runIDs,
			CanonicalRunID:   runIDs[0],
			TopFailureReason: dominantReasonCode(allReasons),
			Drivers:          drivers,
			SeverityScore:    severityScore,
			SeverityLevel:    severityLevel(severityScore),
			ArtifactPointers: artifactPointers,
			Suggestions:      suggestions,
		})
	}

	sort.Slice(families, func(i, j int) bool {
		if families[i].SeverityScore != families[j].SeverityScore {
			return families[i].SeverityScore > families[j].SeverityScore
		}
		if families[i].Count != families[j].Count {
			return families[i].Count > families[j].Count
		}
		return families[i].FamilyID < families[j].FamilyID
	})
	return families
}

func computeRunFingerprint(
	actionSequence []string,
	toolClasses []string,
	targetSystems []string,
	reasonCodeVector []string,
	refReceiptDigests []string,
) (string, error) {
	payload := map[string]any{
		"action_sequence":     actionSequence,
		"tool_classes":        toolClasses,
		"target_systems":      targetSystems,
		"reason_code_vector":  reasonCodeVector,
		"ref_receipt_digests": refReceiptDigests,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal fingerprint payload: %w", err)
	}
	return jcs.DigestJCS(encoded)
}

func loadTraceSignals(paths []string) (map[string][]string, map[string]int, error) {
	reasonsByRun := map[string][]string{}
	traceCountByRun := map[string]int{}
	for _, path := range paths {
		// #nosec G304 -- caller provides explicit local trace paths.
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read trace %s: %w", path, err)
		}
		var trace schemagate.TraceRecord
		if err := json.Unmarshal(raw, &trace); err != nil {
			return nil, nil, fmt.Errorf("parse trace %s: %w", path, err)
		}
		runID := runIDFromTrace(trace)
		if runID == "" {
			continue
		}
		reasons := []string{}
		verdict := normalizeIdentifier(strings.ToLower(strings.TrimSpace(trace.Verdict)))
		if verdict != "" && verdict != "allow" {
			reasons = append(reasons, "trace_verdict_"+verdict)
		}
		for _, violation := range trace.Violations {
			normalizedViolation := normalizeIdentifier(strings.ToLower(strings.TrimSpace(violation)))
			if normalizedViolation == "" {
				continue
			}
			reasons = append(reasons, "violation_"+normalizedViolation)
		}
		reasonsByRun[runID] = uniqueSorted(append(reasonsByRun[runID], reasons...))
		traceCountByRun[runID]++
	}
	return reasonsByRun, traceCountByRun, nil
}

func loadRegressSignals(paths []string) (map[string][]string, error) {
	reasonsByRun := map[string][]string{}
	for _, path := range paths {
		// #nosec G304 -- caller provides explicit local regress result paths.
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read regress result %s: %w", path, err)
		}
		var result schemaregress.RegressResult
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("parse regress result %s: %w", path, err)
		}
		for _, grader := range result.Graders {
			if strings.EqualFold(strings.TrimSpace(grader.Status), regressStatusPass) {
				continue
			}
			runID := runIDFromGrader(grader)
			if runID == "" {
				continue
			}
			reasons := normalizeReasonCodes(grader.ReasonCodes)
			if len(reasons) == 0 {
				reasons = []string{"regress_failure"}
			}
			reasonsByRun[runID] = uniqueSorted(append(reasonsByRun[runID], reasons...))
		}
	}
	return reasonsByRun, nil
}

func normalizeReasonCodes(reasonCodes []string) []string {
	normalized := make([]string, 0, len(reasonCodes))
	for _, reasonCode := range reasonCodes {
		normalizedCode := normalizeIdentifier(strings.ToLower(strings.TrimSpace(reasonCode)))
		if normalizedCode == "" {
			continue
		}
		normalized = append(normalized, normalizedCode)
	}
	return uniqueSorted(normalized)
}

func runIDFromTrace(trace schemagate.TraceRecord) string {
	correlationID := strings.TrimSpace(trace.CorrelationID)
	if runIDPattern.MatchString(correlationID) {
		return runIDPattern.FindString(correlationID)
	}
	traceID := strings.TrimSpace(trace.TraceID)
	if runIDPattern.MatchString(traceID) {
		return runIDPattern.FindString(traceID)
	}
	return ""
}

func runIDFromGrader(grader schemaregress.GraderResult) string {
	if value, ok := grader.Details["run_id"]; ok {
		if runID, ok := value.(string); ok {
			trimmed := strings.TrimSpace(runID)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	name := strings.TrimSpace(grader.Name)
	if name == "" {
		return ""
	}
	segments := strings.Split(name, "/")
	if len(segments) == 0 {
		return ""
	}
	return strings.TrimSpace(segments[0])
}

func classifyToolClass(toolName string) string {
	lower := strings.ToLower(toolName)
	destructiveTokens := []string{"delete", "drop", "destroy", "remove", "purge"}
	for _, token := range destructiveTokens {
		if strings.Contains(lower, token) {
			return "destructive"
		}
	}
	writeTokens := []string{"write", "update", "save", "send", "export", "create"}
	for _, token := range writeTokens {
		if strings.Contains(lower, token) {
			return "write"
		}
	}
	readTokens := []string{"read", "list", "get", "search", "fetch"}
	for _, token := range readTokens {
		if strings.Contains(lower, token) {
			return "read"
		}
	}
	return "execute"
}

func toolClassScore(toolClass string) int {
	switch toolClass {
	case "destructive":
		return 4
	case "write":
		return 3
	case "execute":
		return 2
	case "read":
		return 1
	default:
		return 1
	}
}

func normalizeTargetSystem(sourceType string, locator string) string {
	typePart := normalizeIdentifier(strings.ToLower(strings.TrimSpace(sourceType)))
	locatorPart := normalizeIdentifier(strings.ToLower(strings.TrimSpace(locator)))
	if typePart == "" {
		typePart = "unknown"
	}
	if locatorPart == "" {
		return typePart
	}
	if len(locatorPart) > 48 {
		locatorPart = locatorPart[:48]
	}
	return typePart + ":" + locatorPart
}

func targetSensitivityScore(targetSystems []string) int {
	score := 1
	for _, target := range targetSystems {
		lower := strings.ToLower(target)
		switch {
		case strings.Contains(lower, "prod"),
			strings.Contains(lower, "payment"),
			strings.Contains(lower, "finance"),
			strings.Contains(lower, "customer"),
			strings.Contains(lower, "pii"),
			strings.Contains(lower, "db"):
			if score < 3 {
				score = 3
			}
		case strings.Contains(lower, "internal"),
			strings.Contains(lower, "staging"),
			strings.Contains(lower, "queue"):
			if score < 2 {
				score = 2
			}
		}
	}
	return score
}

func policyPostureScore(reasonCodes []string) int {
	for _, reasonCode := range reasonCodes {
		lower := strings.ToLower(reasonCode)
		if strings.Contains(lower, "policy") ||
			strings.Contains(lower, "approval") ||
			strings.Contains(lower, "blocked") ||
			strings.Contains(lower, "credential") {
			return 2
		}
	}
	return 0
}

func inferDriverCategories(reasonCodes []string) []string {
	drivers := make([]string, 0, 4)
	for _, reasonCode := range reasonCodes {
		lower := strings.ToLower(reasonCode)
		switch {
		case strings.Contains(lower, "policy"),
			strings.Contains(lower, "approval"),
			strings.Contains(lower, "blocked"),
			strings.Contains(lower, "credential"),
			strings.Contains(lower, "violation"):
			drivers = append(drivers, driverPolicyChange)
		case strings.Contains(lower, "diff"),
			strings.Contains(lower, "schema"),
			strings.Contains(lower, "result_status"),
			strings.Contains(lower, "grader"),
			strings.Contains(lower, "unexpected_exit_code"):
			drivers = append(drivers, driverToolResultChange)
		case strings.Contains(lower, "ref"),
			strings.Contains(lower, "receipt"),
			strings.Contains(lower, "retrieval"),
			strings.Contains(lower, "source"):
			drivers = append(drivers, driverReferenceSet)
		case strings.Contains(lower, "config"),
			strings.Contains(lower, "nondeterministic"),
			strings.Contains(lower, "runtime"),
			strings.Contains(lower, "timeout"):
			drivers = append(drivers, driverConfiguration)
		}
	}
	drivers = uniqueSorted(drivers)
	if len(drivers) == 0 {
		return []string{driverConfiguration}
	}
	return drivers
}

func suggestionsForDrivers(drivers []string) []schemascout.SignalFixSuggestion {
	suggestions := make([]schemascout.SignalFixSuggestion, 0, len(drivers))
	for _, driver := range uniqueSorted(drivers) {
		switch driver {
		case driverPolicyChange:
			suggestions = append(suggestions, schemascout.SignalFixSuggestion{
				Kind:        suggestionPolicyFixture,
				Summary:     "Add or tighten a policy fixture and verify with `gait policy test <policy.yaml> <intent.json> --json`.",
				LikelyScope: "policy/*.yaml + examples/policy/intents",
			})
		case driverToolResultChange:
			suggestions = append(suggestions, schemascout.SignalFixSuggestion{
				Kind:        suggestionRegress,
				Summary:     "Add or update a regress fixture, then run `gait regress run --json`.",
				LikelyScope: "fixtures/* + gait.yaml",
			})
		case driverReferenceSet:
			suggestions = append(suggestions, schemascout.SignalFixSuggestion{
				Kind:        suggestionRegress,
				Summary:     "Pin retrieval filters/receipt set and capture a fresh runpack before re-running regress.",
				LikelyScope: "run input + refs receipts",
			})
		case driverConfiguration:
			suggestions = append(suggestions, schemascout.SignalFixSuggestion{
				Kind:        suggestionConfig,
				Summary:     "Pin runtime/model/tool configuration and rerun the incident-to-regress bootstrap path.",
				LikelyScope: "wrapper runtime config + CI env",
			})
		}
	}
	return suggestions
}

func dominantReasonCode(reasonCodes []string) string {
	if len(reasonCodes) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, reasonCode := range reasonCodes {
		counts[reasonCode]++
	}
	dominant := ""
	maxCount := 0
	for reasonCode, count := range counts {
		if count > maxCount || (count == maxCount && (dominant == "" || reasonCode < dominant)) {
			dominant = reasonCode
			maxCount = count
		}
	}
	return dominant
}

func severityLevel(score int) string {
	switch {
	case score >= 150:
		return signalSeverityCritical
	case score >= 110:
		return signalSeverityHigh
	case score >= 70:
		return signalSeverityMedium
	default:
		return signalSeverityLow
	}
}

func normalizeSignalNow(now time.Time) time.Time {
	normalized := now.UTC()
	if normalized.IsZero() {
		return time.Date(defaultSignalTimeYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	return normalized
}

func normalizeProducerVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "0.0.0-dev"
	}
	return trimmed
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
