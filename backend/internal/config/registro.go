package config

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sudameris/backend/internal/crypto"
)

type BancoRegistrado struct {
	URL        string
	PublicKey  ed25519.PublicKey
}

// CargarRegistroBancos lee un JSON {"BCO1": {"url": "...", "clave_publica": "..."}, ...}
// que distribuye el BCP simulado. Si path está vacío, devuelve un mapa
// vacío sin error (todavía no hay registro).
func CargarRegistroBancos(path string) (map[string]BancoRegistrado, error) {
	result := map[string]BancoRegistrado{}
	if path == "" {
		return result, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo registro de bancos: %w", err)
	}

	var raw map[string]struct {
		URL          string `json:"url"`
		ClavePublica string `json:"clave_publica"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parseando registro de bancos: %w", err)
	}

	for bancoID, entry := range raw {
		pub, err := crypto.LoadPublicKey(entry.ClavePublica)
		if err != nil {
			return nil, fmt.Errorf("clave pública inválida para %s: %w", bancoID, err)
		}
		result[bancoID] = BancoRegistrado{URL: entry.URL, PublicKey: pub}
	}

	return result, nil
}
