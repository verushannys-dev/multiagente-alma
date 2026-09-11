// Herramienta de demostración del Clasificador — no es el servicio de
// producción, es un REPL de terminal para mostrar en vivo cómo
// resuelve cada mensaje, campo por campo, y cómo responde cuando
// cambia el estado de un dominio en catalogo_agentes.json.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"multiagente-alma/internal/classifier"
)

func main() {
	catalogoPath := "data/catalogo_agentes.json"
	if len(os.Args) > 1 {
		catalogoPath = os.Args[1]
	}

	cl, err := classifier.New(catalogoPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error iniciando el Clasificador:", err)
		os.Exit(1)
	}

	fmt.Println("Clasificador — Multiagente-Alma (demo)")
	fmt.Println("Catálogo:", catalogoPath)
	fmt.Println("Comandos: 'recargar' (relee el catálogo desde disco), 'salir'")
	fmt.Println(strings.Repeat("-", 60))

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}
		entrada := strings.TrimSpace(scanner.Text())
		if entrada == "" {
			continue
		}

		switch entrada {
		case "salir":
			return
		case "recargar":
			if err := cl.Recargar(); err != nil {
				fmt.Println("Error recargando el catálogo:", err)
				continue
			}
			fmt.Println("Catálogo recargado desde", catalogoPath)
			mostrarEstados(cl)
			continue
		case "estados":
			mostrarEstados(cl)
			continue
		}

		resultado, llmOut, err := cl.Clasificar(entrada)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		fmt.Println("\n--- B2a: lo que devolvió el LLM (sin conocer el estado del dominio) ---")
		imprimirCampo("dominio", llmOut.Dominio)
		imprimirCampo("motivo", llmOut.Motivo)
		imprimirCampo("pregunta", llmOut.Pregunta)
		imprimirCampoLista("opciones", llmOut.Opciones)

		fmt.Println("\n--- B2b: resultado final, tras el lookup del código contra el catálogo ---")
		fmt.Printf("  resultado: %s\n", resultado.Resultado)
		imprimirCampo("dominio", resultado.Dominio)
		imprimirCampo("motivo", resultado.Motivo)
		imprimirCampo("pregunta", resultado.Pregunta)
		imprimirCampoLista("opciones", resultado.Opciones)
	}
}

func mostrarEstados(cl *classifier.Classifier) {
	fmt.Println("Estado actual de cada dominio:")
	for _, a := range cl.Catalogo().Agentes {
		fmt.Printf("  %-25s %s\n", a.ID, a.Estado)
	}
}

func imprimirCampo(nombre string, valor *string) {
	if valor == nil {
		fmt.Printf("  %s: null\n", nombre)
		return
	}
	fmt.Printf("  %s: %q\n", nombre, *valor)
}

func imprimirCampoLista(nombre string, valores []string) {
	if len(valores) == 0 {
		fmt.Printf("  %s: null\n", nombre)
		return
	}
	fmt.Printf("  %s: %v\n", nombre, valores)
}
