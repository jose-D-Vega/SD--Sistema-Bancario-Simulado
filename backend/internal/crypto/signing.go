package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

// LoadPrivateKey acepta tanto la semilla de 32 bytes (formato que da
// openssl) como la clave expandida de 64 bytes (formato interno de Go).
func LoadPrivateKey(b64 string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("clave privada inválida (base64): %w", err)
	}

	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	default:
		return nil, fmt.Errorf("tamaño de clave privada inválido: got %d bytes, want %d o %d",
			len(raw), ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}

func LoadPublicKey(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("clave pública inválida (base64): %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("tamaño de clave pública inválido: got %d, want %d", len(raw), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

func Sign(priv ed25519.PrivateKey, payload []byte) string {
	sig := ed25519.Sign(priv, payload)
	return base64.StdEncoding.EncodeToString(sig)
}

func Verify(pub ed25519.PublicKey, payload []byte, sigB64 string) (bool, error) {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return false, fmt.Errorf("firma inválida (base64): %w", err)
	}
	return ed25519.Verify(pub, payload, sig), nil
}
