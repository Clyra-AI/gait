package actioncontract

import (
	"errors"
	proofcanon "github.com/Clyra-AI/proof/canon"
	"regexp"
	"strings"
)

var canonicalDigestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func canonicalDigest(raw []byte) (string, error) {
	d, e := proofcanon.DigestJCS(raw)
	if e != nil {
		return "", e
	}
	d = "sha256:" + strings.TrimPrefix(d, "sha256:")
	if !canonicalDigestPattern.MatchString(d) {
		return "", errors.New("canonical digest format invalid")
	}
	return d, nil
}
func validCanonicalDigest(d string) bool { return canonicalDigestPattern.MatchString(d) }
