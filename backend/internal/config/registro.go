package config

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sudameris/backend/internal/crypto"
)

// CargarRegistroBancos lee un JSON {"BCO1": "clave_publica_base64", ...}
// que distribuye el BCP simulado y devuelve el mapa listo para el
// middleware de verificación de firma. Si path está vacío (todavía no
// hay registro), devuelve un mapa vacío sin error.
func CargarRegistroBancos(path string) (map[string]ed25519.PublicKey, error) {
	result := map[string]ed25519.PublicKey{}
	if path == "" {
		return result, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo registro de bancos: %w", err)
	}

	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parseando registro de bancos: %w", err)
	}

	for bancoID, pubB64 := range raw {
		pub, err := crypto.LoadPublicKey(pubB64)
		if err != nil {
			return nil, fmt.Errorf("clave pública inválida para %s: %w", bancoID, err)
		}
		result[bancoID] = pub
	}

	return result, nil
}
