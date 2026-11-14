package config

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sudameris/backend/internal/crypto"
)

type BancoRegistrado struct {
	URL       string
	PublicKey ed25519.PublicKey
}

// registroBancosFile refleja el formato oficial que distribuye el BCP
// simulado en 06_registro_bancos_v2.json. Los campos "version",
// "fecha_actualizacion", "algoritmo_firma" y "formato_clave_publica" son
// metadata informativa del archivo; no se usan para cargar el registro,
// pero quedan declarados para que el formato esperado sea explícito.
type registroBancosFile struct {
	Version            int    `json:"version"`
	FechaActualizacion string `json:"fecha_actualizacion"`
	AlgoritmoFirma     string `json:"algoritmo_firma"`
	Bancos             []struct {
		BancoID      string `json:"banco_id"`
		Nombre       string `json:"nombre"`
		URLBase      string `json:"url_base"`
		ClavePublica string `json:"clave_publica"`
	} `json:"bancos"`
}

// CargarRegistroBancos lee el registro de bancos en el formato oficial
// que distribuye el BCP simulado (documento 06: un objeto con un array
// "bancos", cada uno con banco_id, url_base y clave_publica). Si path
// está vacío, devuelve un mapa vacío sin error (todavía no hay registro).
//
// Una entrada sin completar todavía (url_base o clave_publica vacíos, o
// con el placeholder "<...>" del template oficial) se salta en silencio
// en vez de hacer fallar la carga entera: es el caso normal mientras el
// BCP simulado todavía no reunió los datos de los 4 bancos.
func CargarRegistroBancos(path string) (map[string]BancoRegistrado, error) {
	result := map[string]BancoRegistrado{}
	if path == "" {
		return result, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo registro de bancos: %w", err)
	}

	var raw registroBancosFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parseando registro de bancos: %w", err)
	}

	for _, entry := range raw.Bancos {
		if entry.BancoID == "" {
			continue
		}
		if entry.URLBase == "" || entry.ClavePublica == "" ||
			strings.HasPrefix(entry.URLBase, "<") || strings.HasPrefix(entry.ClavePublica, "<") {
			// Entrada todavía sin completar en el template del BCP
			// simulado (placeholder "<...>"); se ignora en vez de fallar.
			continue
		}

		pub, err := crypto.LoadPublicKey(entry.ClavePublica)
		if err != nil {
			return nil, fmt.Errorf("clave pública inválida para %s: %w", entry.BancoID, err)
		}
		result[entry.BancoID] = BancoRegistrado{URL: entry.URLBase, PublicKey: pub}
	}

	return result, nil
}
