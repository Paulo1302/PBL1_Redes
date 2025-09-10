package main

import (
	"fmt"
	"net"
	"time"

	"PBL1_Redes/server"
)

func main() {
	go server.Start()

	// aguarda até que a porta esteja aberta
	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", "localhost:8080", 500*time.Millisecond)
		if err == nil {
			conn.Close()
			fmt.Println("Servidor disponível para testes")
			select {} // bloqueia main para não encerrar
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("Não foi possível iniciar o servidor")
}
