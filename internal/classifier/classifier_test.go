package classifier

import (
	"testing"

	"multiagente-alma/internal/types"
)

func strPtr(s string) *string { return &s }

func TestResolver_DominioActivo(t *testing.T) {
	c := &Classifier{catalogo: types.Catalogo{Agentes: []types.AgenteCatalogo{
		{ID: "turnos", Estado: "activo"},
	}}}
	r := c.resolver(types.LLMOutput{Dominio: strPtr("turnos")})
	if r.Resultado != "agente" {
		t.Fatalf("esperaba 'agente', obtuve %q", r.Resultado)
	}
}

func TestResolver_DominioViaAPI(t *testing.T) {
	c := &Classifier{catalogo: types.Catalogo{Agentes: []types.AgenteCatalogo{
		{ID: "autorizaciones", Estado: "via_api"},
	}}}
	r := c.resolver(types.LLMOutput{Dominio: strPtr("autorizaciones")})
	if r.Resultado != "via_api" {
		t.Fatalf("esperaba 'via_api', obtuve %q", r.Resultado)
	}
}

func TestResolver_SinDominio_NoDisponible(t *testing.T) {
	c := &Classifier{}
	r := c.resolver(types.LLMOutput{})
	if r.Resultado != "no_disponible" {
		t.Fatalf("esperaba 'no_disponible', obtuve %q", r.Resultado)
	}
}

func TestResolver_RequiereAclaracion(t *testing.T) {
	c := &Classifier{}
	r := c.resolver(types.LLMOutput{Motivo: strPtr("multi_intent_distintos_agentes")})
	if r.Resultado != "requiere_aclaracion" {
		t.Fatalf("esperaba 'requiere_aclaracion', obtuve %q", r.Resultado)
	}
}

// Prueba el motivo real de este mensaje: si el mismo dominio cambia de
// estado en el catálogo (simulando 'recargar'), el resultado cambia
// solo, sin tocar código — nada hardcodeado.
func TestResolver_CambioDeEstado_CambiaElResultado(t *testing.T) {
	c := &Classifier{catalogo: types.Catalogo{Agentes: []types.AgenteCatalogo{
		{ID: "recetas", Estado: "via_api"},
	}}}
	r1 := c.resolver(types.LLMOutput{Dominio: strPtr("recetas")})
	if r1.Resultado != "via_api" {
		t.Fatalf("antes del cambio: esperaba 'via_api', obtuve %q", r1.Resultado)
	}

	// Simula un 'recargar' con el dominio ya activo en el catálogo.
	c.catalogo.Agentes[0].Estado = "activo"

	r2 := c.resolver(types.LLMOutput{Dominio: strPtr("recetas")})
	if r2.Resultado != "agente" {
		t.Fatalf("después del cambio: esperaba 'agente', obtuve %q", r2.Resultado)
	}
}
