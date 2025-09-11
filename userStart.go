// File: userStar.go
package main

import (
	"fmt"
	"log"
	"os"
	"PBL1_Redes/user"// Importa o pacote onde está o código do usuário
)

func main() {
	// Verifica se o IP do servidor foi passado como argumento
	if len(os.Args) < 2 {
		log.Fatal("Erro: IP do servidor não fornecido. Uso correto: go run userStar.go <IP_DO_SERVIDOR>")
	}

	// O IP do servidor é o segundo argumento na linha de comando
	serverIP := os.Args[1]

	// Exibe o IP do servidor para confirmar que foi recebido
	fmt.Printf("Conectando ao servidor em %s...\n", serverIP)

	// Chama a função de conexão com o servidor, passando o IP como parâmetro
	if err := user.ServerConn(serverIP); err != nil {
		log.Fatalf("Erro ao executar o jogo: %v", err)
	} else {
		fmt.Println("Jogo finalizado com sucesso!")
	}
}
