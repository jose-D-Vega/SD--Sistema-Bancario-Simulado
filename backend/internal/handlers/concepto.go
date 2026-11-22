package handlers

// limiteConceptoChars acota el texto libre y opcional que el doc 02
// contempla para las transferencias (ver creditoRequest.Concepto en
// interbancario.go). El contrato no define un límite, pero es texto que
// entra por API y queda guardado permanentemente, así que le ponemos uno
// nosotros para no dejarlo sin tope.
const limiteConceptoChars = 140

// conceptoOrNil convierte el string crudo del request al puntero que
// espera models.Operacion: "" se guarda como NULL (no se mandó concepto),
// nunca como un string vacío -- son cosas distintas, y el frontend ya
// decide qué mostrar por default cuando es NULL (ver bank.js:
// `transfer.concept || 'Transferencia enviada'`), así que acá no hace
// falta inventar ninguna etiqueta.
func conceptoOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// conceptoDemasiadoLargo valida el límite para las rutas que SÍ podemos
// rechazar sin romper interoperabilidad: las que arranca nuestro propio
// usuario de banca personal (transferencia interna o interbancaria
// saliente). No se usa para RecibirCredito -- ahí el concepto lo mandó
// OTRO banco, y preferimos truncarlo (ver truncarConcepto) antes que
// rechazar una transferencia de plata real por un detalle cosmético que
// el contrato ni siquiera limita.
func conceptoDemasiadoLargo(s string) bool {
	return len([]rune(s)) > limiteConceptoChars
}

// truncarConcepto recorta el concepto que llega de OTRO banco (en
// RecibirCredito) si se pasa de limiteConceptoChars, en vez de rechazar
// el crédito entero: el doc 02 no define un límite para este campo, así
// que tratarlo como un error de contrato sería imponerle a los otros 3
// bancos una regla que no está escrita en ningún lado.
//
// Trunca por runas, no por bytes: un concepto en español con tildes o
// "ñ" ocupa más de un byte por carácter, y cortar con s[:n] podría partir
// uno de esos caracteres a la mitad y dejar bytes UTF-8 inválidos.
func truncarConcepto(s string) string {
	runas := []rune(s)
	if len(runas) <= limiteConceptoChars {
		return s
	}
	return string(runas[:limiteConceptoChars])
}
