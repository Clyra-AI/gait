package sign

import (
	"crypto/ed25519"
	"fmt"

	"github.com/Clyra-AI/gait/core/jcs"
)

func DigestJSON(input []byte) (string, error) {
	return jcs.DigestJCS(input)
}

func SignJSON(priv ed25519.PrivateKey, input []byte) (Signature, error) {
	digest, err := DigestJSON(input)
	if err != nil {
		return Signature{}, err
	}
	return SignDigestHex(priv, digest)
}

func VerifyJSON(pub ed25519.PublicKey, sig Signature, input []byte) (bool, error) {
	digest, err := DigestJSON(input)
	if err != nil {
		return false, err
	}
	if sig.SignedDigest == "" {
		return false, fmt.Errorf("missing signed_digest")
	}
	if sig.SignedDigest != digest {
		return false, fmt.Errorf("signed_digest mismatch")
	}
	return VerifyDigestHex(pub, sig)
}

func SignManifestJSON(priv ed25519.PrivateKey, manifestJSON []byte) (Signature, error) {
	return SignJSON(priv, manifestJSON)
}

func VerifyManifestJSON(pub ed25519.PublicKey, sig Signature, manifestJSON []byte) (bool, error) {
	return VerifyJSON(pub, sig, manifestJSON)
}

func SignTraceRecordJSON(priv ed25519.PrivateKey, traceJSON []byte) (Signature, error) {
	return SignJSON(priv, traceJSON)
}

func VerifyTraceRecordJSON(pub ed25519.PublicKey, sig Signature, traceJSON []byte) (bool, error) {
	return VerifyJSON(pub, sig, traceJSON)
}
