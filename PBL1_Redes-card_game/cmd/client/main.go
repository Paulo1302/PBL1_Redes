// File: cmd/client/main.go
package main // CORREÇÃO: Pacote alterado para 'main' para ser um executável.

import (
	// CORREÇÃO: Pacotes necessários foram adicionados.
	"fmt"
	"log"
	"os"

	// Importa o seu pacote 'user' para poder usar a função ServerConn.
	"PBL1_Redes-card_game/user"
)

func main() {
	var serverIP string

	// Verifica se um IP foi passado como argumento na linha de comando.
	if len(os.Args) < 2 {
		// Se nenhum IP for fornecido, usa 'localhost' como padrão.
		serverIP = "localhost"
		fmt.Println("Nenhum IP de servidor fornecido. Usando 'localhost' como padrão.")
	} else {
		// Se um IP foi fornecido, usa o que foi informado.
		serverIP = os.Args[1]
	}

	fmt.Printf("Tentando conectar ao servidor em %s:8080...\n", serverIP)

	// CORREÇÃO: A chamada agora especifica o pacote 'user' de onde a função vem.
	if err := user.ServerConn(serverIP); err != nil {
		log.Fatalf("Erro: %v", err)
	} else {
		fmt.Println("Sessão finalizada.")
	}
}