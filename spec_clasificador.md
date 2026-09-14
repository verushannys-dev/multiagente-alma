# Spec: Clasificador (Multiagente-Alma)

> Estado: v1.0 — 10/09/2026.
> Referente: Verónica Elena Mlinarevic
> Editado por: Verónica Elena Mlinarevic, con Claude AI

Primer eslabón de la arquitectura multiagente. El Clasificador **no interpreta el pedido a fondo, no gestiona nivel de acceso, ni ejecuta ninguna acción** — su única responsabilidad es identificar a qué agente-módulo corresponde el mensaje del consultante y derivarlo. Todo lo demás (pedir datos faltantes, validar acceso, resolver la intención) es responsabilidad de cada agente.

---

# A. Reglas de negocio

Lo que se decidió y por qué — independiente de cómo termine representado en código.

Este bloque registra la decisión de negocio en prosa, no su forma técnica: acá va *qué se resolvió y la razón*, no nombres de campos, tipos de dato ni JSON — eso es tarea del Bloque B, que se deriva de este una vez que estas reglas están confirmadas. La separación es deliberada, por dos motivos:

- **El razonamiento sobrevive aunque el contrato técnico cambie.** Si mañana el formato de mensaje entre el Clasificador y los agentes se modifica, el *por qué* de cada regla (ej. por qué el multi-intent entre agentes distintos no sostiene memoria de la intención pendiente) sigue siendo válido y no hay que reconstruirlo desde el código o desde una conversación vieja.
- **Fuerza a que la ambigüedad se resuelva acá, no en el código.** Si al traducir una regla de este bloque al contrato técnico (Bloque B) cuesta encontrar el campo o el tipo que le corresponde, es señal de que la regla no quedó tan clara como parecía — y se vuelve a este bloque a precisarla, en vez de improvisar la respuesta en la implementación.

Por eso cada punto de este bloque es una decisión que ya se discutió y se cerró en conversación — no una propuesta a evaluar mientras se escribe.

## A1. Responsabilidad del Clasificador

- Recibir el mensaje del consultante y determinar a qué agente-módulo corresponde.
- No interpreta el detalle de la intención más allá de lo necesario para decidir el destino — esa interpretación fina (qué especialidad, qué sede, qué beneficiario, etc.) es tarea del agente que la recibe, no del Clasificador.
- No calcula ni transporta nivel de acceso — cada agente gestiona el suyo de forma independiente.
- No sostiene estado de conversación entre mensajes (ver A4) — es, a propósito, la pieza más liviana de la arquitectura.

## A2. Los módulos son agentes, no funciones pasivas

Cada módulo es un **agente de IA** que:

- Interactúa directamente con el consultante si necesita algún dato para gestionar la consulta.
- Gestiona su propio nivel de acceso.
- Ejecuta la acción pertinente una vez que tiene lo que necesita.

El Clasificador entrega el mensaje al agente correspondiente y su intervención en esa consulta termina ahí.

## A3. Multi-intención, mismo agente

Si el mensaje contiene más de una intención pero todas corresponden al mismo agente (ej. dos turnos), el Clasificador deriva una sola vez a ese agente, y es **el propio agente** quien decide qué tomar primero y en qué orden — respetando las reglas de negocio ya establecidas del dominio (ej. el límite de turnos activos por persona). El Clasificador no interviene en ese orden.

## A4. Multi-intención, agentes distintos

Este es el caso **excepción**, no el caso común — la mayoría de los mensajes resuelven en un solo agente, o en varios intents del mismo agente (A3).

Cuando el Clasificador detecta que el mensaje tiene intenciones que corresponden a agentes distintos:

1. El Clasificador pregunta al consultante con cuál de las intenciones quiere empezar.
2. Deriva al agente correspondiente a esa elección.
3. Por definición de gerencia, **cuando ese agente resuelve su intención, la conversación se cierra** — no queda una sesión abierta esperando la segunda intención.
4. El último mensaje del agente orienta al consultante a **rehacer la consulta** por la intención que quedó pendiente, volviendo a pasar por el Clasificador.

**Por qué no se sostiene memoria del intent pendiente:** se evaluó como alternativa (que el sistema recordara la intención pendiente y el consultante la retomara directo en el agente correspondiente), pero se descartó a favor de esta regla más simple, por dos razones:
- Es coherente con la política de cierre de conversación ya definida por gerencia.
- El costo de "recursar" es bajo: al ser un LLM interpretando lenguaje natural, retomar la segunda intención no implica repetir el pedido completo — alcanza con dos o tres palabras (ej. "y lo de laboratorio" alcanza para que el Clasificador retome el contexto del dominio).

Hay un motivo adicional, más de fondo: sostener memoria del intent pendiente no es solo un costo de implementación (una sesión, un TTL, un lugar donde persistir ese dato) — es recuperar, aunque sea en una porción chica, la responsabilidad de coordinación entre agentes que esta arquitectura le sacó a propósito al Clasificador. Si el Clasificador tuviera que recordar una intención pendiente entre mensajes, dejaría de ser "solo identifica a qué agente va" para volver a comportarse, parcialmente, como el Orquestador único de la arquitectura anterior. La decisión traslada ese costo al consultante (repetir dos o tres palabras) en vez de a la infraestructura, precisamente porque ese caso es la excepción, no la regla (ver arriba).

Esto mantiene al Clasificador sin estado de sesión propio — no necesita persistir nada entre el mensaje que detectó el multi-intent y el mensaje de retorno.

## A5. Agente RAG (información general)

Uno de los agentes del catálogo es un **gestor de RAG**, con nivel de acceso requerido **1** (público).

- **Alcance:** cubre información general e institucional — más amplio que un simple módulo de preguntas frecuentes. Incluye datos de referencia de baja frecuencia de cambio (ej. direcciones y horarios generales de sedes).
- **Es la misma pieza** que el proyecto de rediseño de la biblioteca documental institucional de OSEP con arquitectura RAG — no es un esfuerzo paralelo ni una reimplementación propia del prototipo.
- El techo de qué tan amplio es el corpus que puede responder lo define ese proyecto externo, no el prototipo — el Clasificador solo necesita saber que existe como destino válido para consultas de información general.

**Criterio para distinguirlo de un agente de dominio:** el corte no es por tema, es por origen y frescura del dato.
- **Contenido curado manualmente** (no depende de ningún sistema transaccional en vivo) → RAG.
- **Dato que vive en un sistema de gestión que se actualiza solo** (ej. disponibilidad real de una especialidad en una sede, estado de una agenda) → el agente de dominio correspondiente, nunca el RAG, aunque la pregunta *suene* informativa. Ejemplo: "¿tiene cardiología en Maipú?" no va al RAG — depende de la misma agenda en vivo que usa Turnos, así que es Turnos quien la resuelve (ver A6).

## A6. Los agentes no se llaman entre sí — "Sedes" no es un dominio propio

Cada agente resuelve las dependencias que necesita directamente contra la API de fondo correspondiente — no hay coordinación agente-a-agente, ni siquiera para un dato que a primera vista "pertenece" a otro dominio.

Ejemplo concreto: cuando el agente Turnos necesita saber si la sede solicitada ofrece la especialidad pedida, resuelve esa consulta él mismo contra la API de sedes — no delega esa pregunta a un "agente Sedes". Esto aplica como patrón general: cualquier agente que en el futuro necesite datos de sedes hace lo mismo, cada uno directo a la fuente.

Por esto, **"sedes" no es un dominio del catálogo del Clasificador**. Las consultas sueltas de ubicación/horario que son puramente informativas (dirección de una sede, qué sedes hay en una zona) van al RAG (A5); las que dependen de disponibilidad real de un servicio en una sede puntual se resuelven dentro del agente de dominio que las necesita (ej. Turnos), nunca como destino propio.

## A7. Estados de derivación

El Clasificador puede resolver un mensaje de tres formas distintas de cara al consultante:

- **Agente activo** — el dominio identificado ya tiene agente propio implementado. Se deriva directo.
- **`via_api`** — el dominio identificado es una intención real y reconocida del catálogo, y ese dominio sí lo resuelve la API vigente, aunque todavía no tenga agente propio. La consulta se resuelve por la API de ALMA, de forma **transparente para el consultante** (sin aviso de que está pasando por otro canal — agregar ese aviso solo sumaría fricción sin beneficio).
- **`no_disponible`** — el mensaje, de cara al consultante, no tiene ningún destino que lo resuelva hoy. Internamente esto cubre dos situaciones distintas, que el catálogo diferencia con su campo `estado` (B3):
  - **Sin match alguno** — el mensaje no corresponde a ningún dominio del catálogo. Este caso se alcanza recién después de que el Clasificador ya intentó resolver el dominio (incluyendo, si corresponde, ofrecer una aclaración que descarte el dominio más cercano — ver la regla de calibración del prompt) y no pudo ubicarlo en ninguna entrada — no es la respuesta ante la primera señal ambigua.
  - **Dominio identificado, sin nada que lo resuelva** (`estado: "sin_resolver"` en el catálogo) — el mensaje sí corresponde a un dominio real y reconocido, pero ni un agente ni la API vigente lo resuelven hoy (ej. contenido del RAG que el catálogo de la API original no contemplaba). El texto de cortesía que recibe el consultante es el mismo que en el caso anterior, pero el `Resultado` interno mantiene el `dominio` identificado — ver A8.

El manejo posterior a `no_disponible` (mensaje de cortesía, espera de continuar con otra consulta u ofrecer cierre) es responsabilidad de la capa de canal, no del Clasificador — ver Bloque B.

## A8. Registro de consultas no resueltas

Cada `no_disponible` es información valiosa: es la señal más directa de demanda real para la que hoy no existe ningún destino, ni en el prototipo ni en la API vigente — insumo para decidir qué agregar al catálogo a futuro. Este registro (y su eventual notificación, por ejemplo a administradores) es responsabilidad de otra capa, ajena al Clasificador — el Clasificador no necesita saber que esto se está capitalizando, su output ya es suficiente para que quien lo invoca decida loguearlo.

**Esa capa necesita el mensaje original completo, no solo el `Resultado`** — el `dominio` que devuelve el Clasificador es una interpretación forzada contra un catálogo fijo, útil pero no autoritativa: si un patrón de consultas apunta a algo que el catálogo ni siquiera contempla, solo el texto crudo permite detectarlo, no la etiqueta de dominio sola.

**No todo `no_disponible` es la misma señal para gerencia** — al menos tres tipos distintos terminan en este mismo resultado, y el registro debería poder diferenciarlos, no solo contar volumen:
- **Queja o desahogo sin pedido concreto** (ej. "la OSEP no sirve") — no hay ninguna intención de servicio, el catálogo no tiene nada que ver.
- **Necesidad real sin ningún dominio en el catálogo** (`dominio: null` en el `Resultado`) — señal de que podría faltar un dominio nuevo.
- **Dominio identificado pero sin nada que lo resuelva** (`dominio` poblado, `estado: "sin_resolver"` en el catálogo — ver A7) — señal de que ya se sabe qué falta, y con qué urgencia se está pidiendo; útil para priorizar qué construir primero.

---

# B. Contrato técnico

Cómo se representa el Bloque A en el intercambio de datos entre el Clasificador y lo que lo invoca.

> **Nota de notación:** los bloques de código de esta sección usan una notación abreviada para describir el contrato — ej. `"campo": "tipo | null"` para indicar opcionalidad, o `["string"] | null` para un array opcional. No es JSON válido literal, es una forma legible de comunicar el shape del dato. La forma ejecutable real se define en el código (tipos/structs de Go).

## B1. Input esperado

```json
{
  "mensaje": "string (texto libre del consultante)"
}
```

Deliberadamente mínimo: sin historial de sesión, sin contexto previo — coherente con A1 y A4 (el Clasificador no sostiene estado entre mensajes).

## B2. Dos etapas de resolución

El output final (`resultado: agente | via_api | no_disponible | requiere_aclaracion`) no lo decide todo el LLM — se arma en dos pasos, porque `activo` vs. `via_api` no es algo que requiera interpretación de lenguaje, es un dato fijo y determinístico que ya está en `catalogo_agentes.json` (B3). Pedirle al LLM que "decida" ese campo sería exponerlo a equivocarse en algo que en realidad es un lookup simple.

### B2a. Lo que devuelve el LLM

```json
{
  "dominio": "string | null",
  "motivo": "ambiguedad_dominio | multi_intent_distintos_agentes | null",
  "pregunta": "string | null",
  "opciones": ["string"] | null
}
```

El LLM identifica el dominio (o la necesidad de aclaración, o ningún dominio si no hay match) — nunca decide si ese dominio ya tiene agente activo o no. Su tarea termina en clasificar, no en resolver el estado de la migración.

### B2b. Lo que arma el código

Con la respuesta del LLM en mano, el código hace un lookup contra `catalogo_agentes.json` (ya cargado en memoria, B3) y completa el output final que consume quien invocó al Clasificador:

```json
{
  "resultado": "agente | via_api | no_disponible | requiere_aclaracion",
  "dominio": "string | null",
  "motivo": "ambiguedad_dominio | multi_intent_distintos_agentes | null",
  "pregunta": "string | null",
  "opciones": ["string"] | null
}
```

Cómo se completa cada campo según el caso (ver A7 para la definición de negocio de cada estado):

- **Si el LLM devolvió `dominio` y ese `id` existe en el catálogo con `estado: activo`** → `resultado: "agente"`, `dominio` con el `id`, resto en `null`.
- **Si el LLM devolvió `dominio` y ese `id` existe en el catálogo con `estado: via_api`** → `resultado: "via_api"`, `dominio` también poblado — aunque el fallback sea transparente para el consultante (A7), poblarlo permite medir qué dominios siguen derivando a la API vigente, útil para trackear el avance de la migración.
- **Si el LLM devolvió `dominio` y ese `id` existe en el catálogo con `estado: sin_resolver`** → `resultado: "no_disponible"`, pero a diferencia del caso siguiente, **`dominio` queda poblado**, no `null` — es la variante de A7/A8 donde sí se sabe qué se pidió, aunque nada lo resuelva todavía.
- **Si el LLM no devolvió ningún `dominio`** → `resultado: "no_disponible"`, con `dominio: null` — no hubo ningún match, ni siquiera parcial. El mensaje de cortesía hacia el consultante no lo redacta el Clasificador — es un texto fijo que vive en la capa de canal (ver A7, A8), y es el mismo texto sea cual sea la variante de `no_disponible`.
- **Si el LLM devolvió `motivo` en lugar de (o adicionalmente a) un `dominio` resuelto** → `resultado: "requiere_aclaracion"`, tal como lo definió el LLM directamente (esto sí lo decide el modelo, no el código — es interpretación de lenguaje, no un lookup). `motivo` distingue si la ambigüedad es de a qué dominio corresponde (`ambiguedad_dominio`, ej. "credencial" solapado entre dominios) o si son intenciones claramente distintas que requieren elegir con cuál empezar (`multi_intent_distintos_agentes`, ver A4). `pregunta` y `opciones` llevan lo necesario para mostrar algo concreto al consultante — nunca una pregunta abierta.

El Clasificador puede formular una pregunta de aclaración únicamente para resolver a qué dominio deriva — nunca para recolectar datos que el agente de destino vaya a necesitar después (eso es tarea del agente, no del Clasificador — A1). El prompt del sistema prefiere intentar una aclaración que confirme o descarte un dominio cercano antes de rendirse a `dominio: null` — ver A7.

## B3. Banco de verdad que consume

`catalogo_agentes.json` — la lista de dominios válidos, con su `id`, `nombre`, `descripcion` (la que usa el LLM para matchear el mensaje) y `estado` (`activo` | `via_api` | `sin_resolver` — ver A7 para qué significa cada uno). Se carga una sola vez al arrancar el proceso; un cambio de `estado` (ej. cuando un agente nuevo pasa a `activo`) requiere redeploy, no hot-reload — no hace falta más que eso, porque tanto la API vigente como los futuros agentes resuelven contra el mismo recurso de fondo, así que un `estado` dinámico no aportaría resiliencia ante fallas de ese recurso.

No incluye nivel de acceso — eso es responsabilidad exclusiva de cada agente (A1).

---

# C. Casos de prueba

| # | Entrada del usuario | Resultado esperado | Estado en código |
|---|---|---|---|
| 1 | "papanicolau" (sustantivo suelto, sin verbo) | `resultado: "requiere_aclaracion"`, `motivo: "ambiguedad_dominio"`, `opciones: [turnos, resultados_tramites, autorizaciones]` | ✅ probado (REPL) |
| 2 | "necesito un pap" | `resultado: "requiere_aclaracion"`, `motivo: "ambiguedad_dominio"`, `opciones: [turnos, resultados_tramites]` | ✅ probado (REPL) |
| 3 | "que horario tiene la sede de Maipú" | `resultado: "no_disponible"`, `dominio: "rag"` — dominio bien identificado, pero `rag` pasó a `estado: "sin_resolver"` tras esta sesión de pruebas | ⚠️ probado contra estado anterior (daba `via_api`); pendiente re-correr contra el estado actual |
| 4 | "como puedo llamar a la ambulancia" | `resultado: "no_disponible"` | ✅ probado (REPL) — ver D: gap de emergencias |
| 5 | "ambulancia" (sustantivo suelto) | `resultado: "requiere_aclaracion"`, `motivo: "ambiguedad_dominio"`, `opciones: [autorizaciones, rag]` | ✅ probado (REPL) — mismo concepto que el caso 4, resultado distinto por la forma verbal (ver nota abajo) |
| 6 | "quiero ver mis resulados de anali" (con errores de tipeo) | `resultado: "via_api"`, `dominio: "resultados_tramites"` | ✅ probado (REPL) — tolera el error tipográfico sin pedir aclaración |
| 7 | "la osep no sirve" (queja sin pedido concreto) | `resultado: "no_disponible"` | ✅ probado (REPL) — correcto: no hay ninguna intención de servicio en el mensaje, distinto de un gap de dominio |
| 8 | "tengo a mi suegro con internación domiciliaria, como hago" | `resultado: "no_disponible"` | ✅ probado (REPL) — ver D: gap de internación domiciliaria |

**Notas sobre estos casos:**

- **Sensibilidad al verbo, no solo al sustantivo (casos 1, 2, 4, 5).** Un sustantivo suelto sin verbo (caso 1, 5) suele admitir varias lecturas del catálogo a la vez → `requiere_aclaracion`. El mismo concepto con un verbo específico que no calza con ninguna descripción existente (caso 2 acota bien; caso 4, "llamar", no calza con nada) puede saltar directo a `no_disponible` en vez de intentar una aclaración — es una fragilidad real de clasificar por lenguaje natural contra descripciones fijas: pequeños cambios de redacción cruzan el umbral entre "no encuentro nada" y "encuentro algo ambiguo", aunque la necesidad de fondo sea la misma (ver D).
- **B2a y B2b salen idénticos cuando `resultado: "requiere_aclaracion"`.** No es un bug — es el comportamiento esperado de B2b: para este resultado, el código no transforma nada, porque `motivo`/`pregunta`/`opciones` ya los decidió el LLM directamente (es interpretación de lenguaje, no un lookup contra el catálogo). La traducción entre lo que devuelve el LLM y el resultado final solo aplica a `agente`/`via_api` (donde sí hay un cruce contra `estado` en el catálogo) y a `no_disponible`.
- **No todo `no_disponible` es del mismo tipo (casos 4, 7, 8).** El caso 7 es un `no_disponible` "limpio": no hay ninguna intención de servicio en el mensaje, el catálogo no tiene nada que ver. Los casos 4 y 8 son distintos: hay una necesidad real, pero el catálogo no tiene (o perdió, al resumir la descripción) ningún dominio que la cubra. A8 necesita distinguir estos dos tipos de señal para que el registro le sirva a gerencia (ver D).
- **Que `autorizaciones` aparezca como opción no implica que el sistema haya determinado que el Papanicolau requiere autorización.** Es una opción ofrecida por posibilidad lingüística del sustantivo suelto ("papanicolau" admite el verbo "autorizar" tanto como "sacar turno" o "consultar resultado"), no una inferencia sobre una regla de negocio real — el Clasificador no tiene, ni debería tener, acceso a un catálogo de qué prácticas requieren autorización (eso es responsabilidad del futuro agente Autorizaciones, contra su propio banco de verdad, ver D).

---

# D. Pendientes / gaps abiertos

- **Segmentación del mensaje en casos multi-agente:** cuando el Clasificador deriva la intención elegida (A4), ¿le pasa al agente el mensaje completo del consultante, o solo el fragmento correspondiente a esa intención? Sin resolver todavía.
- **Solapamientos entre dominios del catálogo** (ej. Autorizaciones vs. Trámites administrativos): se tratarán cuando se diseñe el agente de cada dominio puntual, no en esta etapa.
- **Fusión de Credencial dentro de Afiliación:** cuando se diseñe el agente Afiliación, evaluar si Credencial se absorbe como una acción más de ese agente o se mantiene como dominio separado del catálogo. Hasta entonces, `credencial` sigue como entrada propia del catálogo.
- **Trade-off de Sedes como capacidad interna:** ver A6 — no es un gap de negocio, sino algo a tener presente cuando se diseñe cada agente que consuma datos de sedes, para no reconstruir coordinación entre agentes por otra vía.
- **Gap de internación domiciliaria:** "tengo a mi suegro con internación domiciliaria, como hago" da `no_disponible` (variante sin match, `dominio: null`) hoy. El catálogo original de módulos tenía una pista tenue ("Autoriz. Adulto Hosp" en Autorizaciones) que se perdió al resumir la descripción de `autorizaciones` para este catálogo. Pendiente confirmar (recordatorio pedido para el lunes): si "Adulto Hosp" se refiere a lo mismo que el proyecto separado ADI (internación domiciliaria) o es una autorización distinta, y si corresponde ampliar la descripción de `autorizaciones` para cubrirlo explícitamente, darle su propio dominio, o mantenerlo como gap aparte.
- **Casos de prueba adicionales:** el Bloque C tiene 8 casos verificados contra la API real (REPL) — faltan casos para `resultado: "agente"` (Turnos, el único dominio activo) probados explícitamente, para el nuevo `estado: "sin_resolver"` de `rag` contra la API real, y para multi-intent entre agentes distintos (A4).

**Aplicado en esta tanda (ya no pendiente):** el tercer valor de `estado` en el catálogo (`sin_resolver`, ver A7/B3), la calibración del prompt para intentar `requiere_aclaracion` antes de rendirse a `dominio: null` (ver A7/B2b), y la distinción de tipos de `no_disponible` en el registro (A8).


