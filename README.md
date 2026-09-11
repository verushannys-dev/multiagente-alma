# Multiagente-Alma

Prototipo del Clasificador — ver `specs/spec_clasificador.md` para las reglas de negocio y el contrato técnico completos. Este README cubre solo los gaps de código (regla de `METODOLOGIA_SDD.md` sección 4: los gaps de diseño/negocio viven en la spec, no acá).

## Requisitos

- Go 1.22+
- Variable de entorno `ANTHROPIC_API_KEY` (configurala como secret del Codespace, no la pegues en ningún archivo del repo).
- Opcional: `ANTHROPIC_MODEL` (por defecto usa `claude-sonnet-5`).

## Correr el REPL de demostración

```bash
go run ./cmd/repl
```

Escribís un mensaje y ves dos bloques:
- **B2a** — lo que devolvió el LLM (solo `dominio`/`motivo`/`pregunta`/`opciones`, sin saber si ese dominio está activo o no).
- **B2b** — el resultado final, después de que el código cruza esa respuesta contra `data/catalogo_agentes.json`.

Comandos dentro del REPL:
- `estados` — muestra el estado actual (`activo` | `via_api`) de cada dominio del catálogo cargado.
- `recargar` — vuelve a leer `data/catalogo_agentes.json` desde disco, sin reiniciar el proceso. Pensado para la demostración: editás el `estado` de un dominio en el archivo, corrés `recargar`, y repetís el mismo mensaje para mostrar que la respuesta cambia sin tocar código.
  - **Importante:** este hot-reload es una capacidad de esta herramienta de demo, no una decisión de arquitectura para producción — ver la nota en `internal/classifier/classifier.go` (función `Recargar`) y B3 de la spec.
- `salir`

## Tests

```bash
go test ./... -v
```

`internal/classifier/classifier_test.go` prueba la función `resolver` (B2b) sin llamar a la API real — incluye un caso (`TestResolver_CambioDeEstado_CambiaElResultado`) que verifica explícitamente que el mismo dominio devuelve un resultado distinto si su `estado` en el catálogo cambia, sin que el código de resolución se toque.

## Estructura

```
data/catalogo_agentes.json     ← banco de verdad, ver B3 de la spec
internal/types/types.go        ← contratos (B2a, B2b, Catalogo)
internal/classifier/           ← lógica del Clasificador
cmd/repl/                      ← herramienta de demostración (no es el servicio de producción)
```

## Gaps de código conocidos

- No hay un `cmd/server` real todavía (API HTTP) — solo el REPL de demo. Se agrega cuando haga falta exponer esto como servicio.
- El schema del tool call fuerza el enum de `dominio` contra los ids del catálogo al momento de construir el prompt — si el catálogo cambia sus ids, el schema los sigue automáticamente (no hay lista hardcodeada de dominios en `classifier.go`).
