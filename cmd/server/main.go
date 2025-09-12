// File: serverStart.go
package main // CORREÇÃO: Pacote alterado para 'main' para ser um programa executável.

import (
	// CORREÇÃO: Pacotes que estavam faltando foram importados.
	"fmt"
	"log"
	"net"
	"time"

	// Importa o seu pacote 'server' para poder usar a função Start.
	"PBL1_Redes-card_game/server"
)

func main() {
	// CORREÇÃO: A chamada agora especifica o pacote 'server' de onde a função vem.
	shutdown := server.Start()
	log.Println("Server start process initiated.")

	// Loop para verificar se o servidor está realmente escutando na porta.
	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", "localhost:8080", 500*time.Millisecond)
		if err == nil {
			conn.Close()
			fmt.Println("Servidor iniciado e escutando na porta 8080.")
			fmt.Println("Pressione Ctrl+C para encerrar.")
			// Bloqueia para manter o servidor em execução.
			select {}
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("Falha ao confirmar o início do servidor.")
	shutdown()
}