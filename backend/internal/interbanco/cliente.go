// Package interbanco implementa el lado "origen" del protocolo del doc
// 02: llamadas HTTP salientes a otros bancos, firmadas con nuestra
// clave Ed25519.
package interbanco

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/sudameris/backend/internal/crypto"
)

type Cliente struct {
	BancoID    string
	PrivateKey ed25519.PrivateKey
	HTTPClient *http.Client
}

func NuevoCliente(bancoID string, priv ed25519.PrivateKey) *Cliente {
	return &Cliente{
		BancoID:    bancoID,
		PrivateKey: priv,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
}

type CreditoPayload struct {
	TransferID    string `json:"transfer_id"`
	CuentaDestino string `json:"cuenta_destino"`
	BancoOrigen   string `json:"banco_origen"`
	CuentaOrigen  string `json:"cuenta_origen"`
	Importe       int64  `json:"importe"`
	Moneda        string `json:"moneda"`
	Concepto      string `json:"concepto"`
}

type RespuestaCredito struct {
	TransferID    string `json:"transfer_id"`
	Estado        string `json:"estado"`
	MotivoRechazo string `json:"motivo_rechazo"`
}

// ErrorInterbancario es una respuesta de error explícita del banco
// destino (4xx/5xx con el formato del catálogo), a diferencia de un
// error de red/timeout donde no sabemos si el otro banco llegó a
// procesar la solicitud o no.
type ErrorInterbancario struct {
	Codigo  string
	Mensaje string
	Status  int
}

func (e *ErrorInterbancario) Error() string {
	return fmt.Sprintf("%s (%d): %s", e.Codigo, e.Status, e.Mensaje)
}

func (c *Cliente) EnviarCredito(ctx context.Context, baseURL string, payload CreditoPayload) (*RespuestaCredito, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("armando payload: %w", err)
	}
	return c.request(ctx, http.MethodPost, baseURL, "/interbancario/creditos", payload.TransferID, body)
}

func (c *Cliente) ConsultarCredito(ctx context.Context, baseURL, transferID string) (*RespuestaCredito, error) {
	return c.request(ctx, http.MethodGet, baseURL, "/interbancario/creditos/"+transferID, transferID, nil)
}

// CuentaRemota es la respuesta de GET /cuentas/:numero_cuenta de OTRO
// banco. A propósito solo trae lo que el doc 02 expone en esa consulta
// (nunca el saldo): alcanza para resolver un destinatario antes de
// transferir, sin filtrar información que ese banco no comparte.
type CuentaRemota struct {
	BancoID      string `json:"banco_id"`
	NumeroCuenta string `json:"numero_cuenta"`
	Titular      string `json:"titular"`
	Estado       string `json:"estado"`
}

// ConsultarCuenta llama a GET /cuentas/:numero_cuenta de otro banco (la
// misma ruta firmada que usa el cajero/otro banco para verificar que una
// cuenta existe). Se usa para resolver el destinatario de una
// transferencia interbancaria ANTES de ejecutarla (ver
// handlers.ResolverDestino, banca personal) — es de solo lectura, no
// reserva ni mueve nada.
func (c *Cliente) ConsultarCuenta(ctx context.Context, baseURL, numeroCuenta string) (*CuentaRemota, error) {
	// No hay una clave de idempotencia natural para una consulta de solo
	// lectura (no hay nada que "reintentar sin duplicar"); el header lo
	// exige igual VerifyBankSignature de parte del banco destino, así que
	// generamos una al vuelo.
	idempotencyKey := uuid.NewString()

	path := "/cuentas/" + numeroCuenta
	timestamp := time.Now().UTC().Format(time.RFC3339)
	canonical := crypto.CanonicalString(http.MethodGet, path, c.BancoID, idempotencyKey, timestamp, nil)
	signature := crypto.Sign(c.PrivateKey, canonical)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, bytes.NewReader([]byte{}))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Bank-Id", c.BancoID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Idempotency-Key", idempotencyKey)
	req.Header.Set("X-Signature", signature)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err // error de red/timeout: el llamador decide cómo reaccionar
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var parsed struct {
			Error   string `json:"error"`
			Mensaje string `json:"mensaje"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&parsed) // si no se puede parsear, igual devolvemos el status
		return nil, &ErrorInterbancario{Codigo: parsed.Error, Mensaje: parsed.Mensaje, Status: resp.StatusCode}
	}

	var cuenta CuentaRemota
	if err := json.NewDecoder(resp.Body).Decode(&cuenta); err != nil {
		return nil, fmt.Errorf("respuesta inválida del banco destino: %w", err)
	}
	return &cuenta, nil
}

func (c *Cliente) request(ctx context.Context, method, baseURL, path, idempotencyKey string, body []byte) (*RespuestaCredito, error) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	canonical := crypto.CanonicalString(method, path, c.BancoID, idempotencyKey, timestamp, body)
	signature := crypto.Sign(c.PrivateKey, canonical)

	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bank-Id", c.BancoID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Idempotency-Key", idempotencyKey)
	req.Header.Set("X-Signature", signature)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err // error de red/timeout: el llamador decide cómo reaccionar
	}
	defer resp.Body.Close()

	var parsed struct {
		TransferID    string `json:"transfer_id"`
		Estado        string `json:"estado"`
		MotivoRechazo string `json:"motivo_rechazo"`
		Error         string `json:"error"`
		Mensaje       string `json:"mensaje"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("respuesta inválida del banco destino: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrorInterbancario{Codigo: parsed.Error, Mensaje: parsed.Mensaje, Status: resp.StatusCode}
	}
	return &RespuestaCredito{TransferID: parsed.TransferID, Estado: parsed.Estado, MotivoRechazo: parsed.MotivoRechazo}, nil
}
