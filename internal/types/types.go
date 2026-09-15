// Package types define los contratos compartidos del Clasificador,
// tal como quedaron cerrados en specs/spec_clasificador.md (Bloque B).
package types

// LLMOutput es lo que devuelve el modelo — B2a de spec_clasificador.md.
// El LLM solo identifica dominio (o la necesidad de aclaración); nunca
// decide si ese dominio tiene agente activo o va vía API — eso es un
// lookup determinístico que resuelve el código (ver Resultado, B2b).
type LLMOutput struct {
	Dominio  *string  `json:"dominio"`
	Motivo   *string  `json:"motivo"` // "ambiguedad_dominio" | "multi_intent_distintos_agentes" | null
	Pregunta *string  `json:"pregunta"`
	Opciones []string `json:"opciones"`
}

// Resultado es el output final que recibe quien invoca al Clasificador —
// B2b de spec_clasificador.md. Se arma cruzando LLMOutput contra el
// catálogo (Catalogo.Agentes) — no lo decide el LLM directamente, salvo
// en el caso "requiere_aclaracion", que sí es interpretación de lenguaje.
type Resultado struct {
	Resultado string   `json:"resultado"` // "agente" | "via_api" | "no_disponible" | "requiere_aclaracion"
	Dominio   *string  `json:"dominio"`
	Motivo    *string  `json:"motivo"`
	Pregunta  *string  `json:"pregunta"`
	Opciones  []string `json:"opciones"`
}

// AgenteCatalogo es una entrada de catalogo_agentes.json — un dominio
// del catálogo del Clasificador. No incluye nivel de acceso a propósito
// (A1 de spec_clasificador.md): eso es responsabilidad de cada agente,
// no del Clasificador.
type AgenteCatalogo struct {
	ID          string `json:"id"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
	Estado      string `json:"estado"` // "activo" | "via_api" | "sin_resolver"
}

// Catalogo es el banco de verdad completo — B3 de spec_clasificador.md.
type Catalogo struct {
	Meta    map[string]string `json:"_meta"`
	Nota    string            `json:"_nota"`
	Agentes []AgenteCatalogo  `json:"agentes"`
}
