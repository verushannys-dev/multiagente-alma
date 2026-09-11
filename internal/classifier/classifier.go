// Package classifier implementa el Clasificador descrito en
// specs/spec_clasificador.md. Dos etapas, tal como quedó definido en B2:
//
//   - B2a: el LLM identifica dominio (o la necesidad de aclaración),
//     nunca decide si ese dominio tiene agente activo o va vía API.
//   - B2b: el código hace el lookup contra el catálogo y arma el
//     resultado final.
package classifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"multiagente-alma/internal/types"
)

const anthropicURL = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

// Classifier mantiene el catálogo cargado en memoria y el cliente HTTP
// hacia la API de Claude.
type Classifier struct {
	catalogo     types.Catalogo
	catalogoPath string
	apiKey       string
	model        string
	httpClient   *http.Client
}

// New crea el Clasificador, cargando el catálogo desde catalogoPath.
// Requiere ANTHROPIC_API_KEY en el entorno.
func New(catalogoPath string) (*Classifier, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("falta la variable de entorno ANTHROPIC_API_KEY")
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-sonnet-5"
	}
	c := &Classifier{
		catalogoPath: catalogoPath,
		apiKey:       apiKey,
		model:        model,
		httpClient:   &http.Client{},
	}
	if err := c.Recargar(); err != nil {
		return nil, err
	}
	return c, nil
}

// Recargar vuelve a leer catalogo_agentes.json desde disco, sin
// reiniciar el proceso.
//
// NOTA — esto es una capacidad de esta herramienta de demostración,
// no una decisión de arquitectura para producción: B3 de
// spec_clasificador.md fija que el catálogo se carga una sola vez al
// arrancar, porque un estado dinámico no aporta resiliencia (API
// vigente y futuros agentes comparten el mismo recurso de fondo). El
// propósito de Recargar acá es otro: permitir demostrar en vivo, sin
// reiniciar nada "sospechoso" entre medio, que el comportamiento del
// Clasificador sigue al contenido real del archivo — no a un valor
// fijo en el código.
func (c *Classifier) Recargar() error {
	data, err := os.ReadFile(c.catalogoPath)
	if err != nil {
		return fmt.Errorf("leyendo catálogo: %w", err)
	}
	var cat types.Catalogo
	if err := json.Unmarshal(data, &cat); err != nil {
		return fmt.Errorf("parseando catálogo: %w", err)
	}
	c.catalogo = cat
	return nil
}

// Catalogo expone el catálogo actualmente cargado (solo lectura).
func (c *Classifier) Catalogo() types.Catalogo {
	return c.catalogo
}

// Clasificar manda el mensaje al LLM y devuelve el resultado final ya
// resuelto contra el catálogo (B2b).
func (c *Classifier) Clasificar(mensaje string) (types.Resultado, types.LLMOutput, error) {
	llmOut, err := c.llamarLLM(mensaje)
	if err != nil {
		return types.Resultado{}, types.LLMOutput{}, err
	}
	return c.resolver(llmOut), llmOut, nil
}

// resolver implementa B2b: el lookup determinístico contra el catálogo.
// El LLM nunca decide "activo" vs. "via_api" — eso lo hace esta función.
func (c *Classifier) resolver(out types.LLMOutput) types.Resultado {
	// requiere_aclaracion: lo decidió el LLM directamente (A4/B2), no
	// hay lookup que hacer.
	if out.Motivo != nil {
		return types.Resultado{
			Resultado: "requiere_aclaracion",
			Motivo:    out.Motivo,
			Pregunta:  out.Pregunta,
			Opciones:  out.Opciones,
		}
	}

	// Sin dominio: no hubo match en el catálogo — no_disponible (A7).
	if out.Dominio == nil || *out.Dominio == "" {
		return types.Resultado{Resultado: "no_disponible"}
	}

	// Con dominio: lookup del estado real en el catálogo cargado en
	// memoria — esto es lo que demuestra que no está hardcodeado: si
	// el catálogo cambia (Recargar) y el estado de ese dominio cambia,
	// esta misma función devuelve otro resultado sin que el código
	// que sigue se toque.
	for _, agente := range c.catalogo.Agentes {
		if agente.ID == *out.Dominio {
			if agente.Estado == "activo" {
				return types.Resultado{Resultado: "agente", Dominio: out.Dominio}
			}
			return types.Resultado{Resultado: "via_api", Dominio: out.Dominio}
		}
	}

	// El LLM devolvió un id que no existe en el catálogo — no debería
	// pasar si el prompt está bien armado (ver promptSistema), pero se
	// trata como no_disponible en vez de romper, por seguridad.
	return types.Resultado{Resultado: "no_disponible"}
}

// promptSistema arma el prompt con las reglas de A1-A8 resumidas y la
// lista de dominios del catálogo — sin exponer el campo "estado": el
// LLM no necesita saberlo, esa decisión es del código (B2).
func (c *Classifier) promptSistema() string {
	var sb strings.Builder
	sb.WriteString(`Sos el Clasificador de un sistema multiagente de atención al afiliado de una obra social.

Tu única responsabilidad es identificar a qué dominio corresponde el mensaje del consultante. No interpretás el detalle de la intención más allá de eso, no gestionás nivel de acceso, no ejecutás ninguna acción — eso es tarea del agente de destino.

Reglas:
- Si el mensaje tiene varias intenciones que corresponden al MISMO dominio, igual devolvés un solo dominio — el agente de destino decide el orden internamente.
- Si el mensaje tiene intenciones que corresponden a dominios DISTINTOS, no elijas uno: devolvé motivo="multi_intent_distintos_agentes", con una pregunta y dos opciones (una por cada dominio) para que el consultante elija con cuál empezar.
- Si el mensaje es ambiguo sobre a qué dominio corresponde (no por tener varias intenciones, sino porque no queda claro cuál de dos o más dominios es el correcto), devolvé motivo="ambiguedad_dominio", con una pregunta y las opciones en juego.
- Solo preguntás para resolver A QUÉ DOMINIO deriva — nunca para juntar datos que el agente de destino vaya a necesitar después.
- Si el mensaje no corresponde a ningún dominio de la lista, no inventes uno: dejá dominio en null.
- "Sedes" no es un dominio: consultas de ubicación/horario general van a "rag"; consultas de disponibilidad real de un servicio en una sede (ej. "¿tiene cardiología en Maipú?") van al dominio de acción correspondiente (ej. "turnos"), porque dependen de la misma agenda en vivo que ese agente ya consulta.

Dominios disponibles:
`)
	for _, a := range c.catalogo.Agentes {
		sb.WriteString(fmt.Sprintf("- id: %q — %s: %s\n", a.ID, a.Nombre, a.Descripcion))
	}
	return sb.String()
}

// dominiosEnum devuelve los ids del catálogo, para forzar el schema
// del tool call a solo aceptar dominios que existen de verdad.
func (c *Classifier) dominiosEnum() []string {
	ids := make([]string, len(c.catalogo.Agentes))
	for i, a := range c.catalogo.Agentes {
		ids[i] = a.ID
	}
	return ids
}

// --- Llamado a la API de Claude, con salida forzada a schema (tool use) ---

type anthropicRequest struct {
	Model      string              `json:"model"`
	MaxTokens  int                 `json:"max_tokens"`
	System     string              `json:"system"`
	Messages   []anthropicMessage  `json:"messages"`
	Tools      []anthropicTool     `json:"tools"`
	ToolChoice anthropicToolChoice `json:"tool_choice"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type anthropicResponse struct {
	Content []struct {
		Type  string          `json:"type"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Classifier) llamarLLM(mensaje string) (types.LLMOutput, error) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"dominio": map[string]interface{}{
				"type":        []string{"string", "null"},
				"enum":        append(c.dominiosEnum(), ""), // "" no se usa; null es el valor real de ausencia
				"description": "El id del dominio correspondiente, o null si no hay match.",
			},
			"motivo": map[string]interface{}{
				"type": []string{"string", "null"},
				"enum": []string{"ambiguedad_dominio", "multi_intent_distintos_agentes"},
			},
			"pregunta": map[string]interface{}{
				"type": []string{"string", "null"},
			},
			"opciones": map[string]interface{}{
				"type":  []string{"array", "null"},
				"items": map[string]interface{}{"type": "string"},
			},
		},
		"required": []string{"dominio", "motivo", "pregunta", "opciones"},
	}

	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: 1024,
		System:    c.promptSistema(),
		Messages: []anthropicMessage{
			{Role: "user", Content: mensaje},
		},
		Tools: []anthropicTool{
			{
				Name:        "clasificar",
				Description: "Devuelve la clasificación del mensaje del consultante según las reglas del sistema.",
				InputSchema: schema,
			},
		},
		ToolChoice: anthropicToolChoice{Type: "tool", Name: "clasificar"},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return types.LLMOutput{}, fmt.Errorf("armando request: %w", err)
	}

	req, err := http.NewRequest("POST", anthropicURL, bytes.NewReader(body))
	if err != nil {
		return types.LLMOutput{}, fmt.Errorf("armando request HTTP: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.LLMOutput{}, fmt.Errorf("llamando a la API: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.LLMOutput{}, fmt.Errorf("leyendo respuesta: %w", err)
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return types.LLMOutput{}, fmt.Errorf("parseando respuesta (status %d): %s", resp.StatusCode, string(raw))
	}
	if apiResp.Error != nil {
		return types.LLMOutput{}, fmt.Errorf("error de la API: %s", apiResp.Error.Message)
	}

	for _, block := range apiResp.Content {
		if block.Type == "tool_use" && block.Name == "clasificar" {
			var out types.LLMOutput
			if err := json.Unmarshal(block.Input, &out); err != nil {
				return types.LLMOutput{}, fmt.Errorf("parseando input del tool: %w", err)
			}
			return out, nil
		}
	}
	return types.LLMOutput{}, fmt.Errorf("la respuesta no incluyó el tool_use esperado")
}
