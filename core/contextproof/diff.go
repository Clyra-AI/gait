package contextproof

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Clyra-AI/gait/core/jcs"
	schemarunpack "github.com/Clyra-AI/gait/core/schema/v1/runpack"
)

func ClassifyRefsDrift(left schemarunpack.Refs, right schemarunpack.Refs) (string, bool, bool, error) {
	leftNorm, err := NormalizeRefs(left)
	if err != nil {
		return "", false, false, err
	}
	rightNorm, err := NormalizeRefs(right)
	if err != nil {
		return "", false, false, err
	}

	leftFull, err := refsDigest(leftNorm)
	if err != nil {
		return "", false, false, err
	}
	rightFull, err := refsDigest(rightNorm)
	if err != nil {
		return "", false, false, err
	}

	if leftFull == rightFull &&
		leftNorm.ContextSetDigest == rightNorm.ContextSetDigest &&
		leftNorm.ContextEvidenceMode == rightNorm.ContextEvidenceMode &&
		leftNorm.ContextRefCount == rightNorm.ContextRefCount {
		return driftNone, false, false, nil
	}

	leftRuntimeComparable := runtimeComparableRefs(leftNorm)
	rightRuntimeComparable := runtimeComparableRefs(rightNorm)
	leftRuntime, err := refsDigest(leftRuntimeComparable)
	if err != nil {
		return "", false, false, err
	}
	rightRuntime, err := refsDigest(rightRuntimeComparable)
	if err != nil {
		return "", false, false, err
	}
	if leftRuntime == rightRuntime &&
		leftNorm.ContextEvidenceMode == rightNorm.ContextEvidenceMode &&
		leftNorm.ContextRefCount == rightNorm.ContextRefCount {
		return driftRuntimeOnly, true, true, nil
	}
	return driftSemantic, true, false, nil
}

func runtimeComparableRefs(refs schemarunpack.Refs) schemarunpack.Refs {
	comparable := refs
	comparable.ContextSetDigest = ""
	for i := range comparable.Receipts {
		comparable.Receipts[i].RetrievedAt = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	return comparable
}

func refsDigest(refs schemarunpack.Refs) (string, error) {
	raw, err := json.Marshal(refs)
	if err != nil {
		return "", fmt.Errorf("marshal refs: %w", err)
	}
	digest, err := jcs.DigestJCS(raw)
	if err != nil {
		return "", fmt.Errorf("digest refs: %w", err)
	}
	return digest, nil
}
