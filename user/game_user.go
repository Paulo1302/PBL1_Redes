package user

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type Message struct {
	Sender   string `json:"user"`
	Receiver string `json:"to"`
	Content  string `json:"content"`
}

func ServerConn() error {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		return fmt.Errorf("erro ao conectar no servidor: %w", err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	// mede tempo de ida/volta
	start := time.Now()

	// envia PING
	ping := Message{
		Sender:   "Cliente1",
		Receiver: "Servidor",
		Content:  "PING",
	}
	if err := encoder.Encode(ping); err != nil {
		return fmt.Errorf("erro ao enviar ping: %w", err)
	}

	// espera respostas
	for i := 0; i < 2; i++ {
		var resp Message
		err = decoder.Decode(&resp)
		if err != nil {
			return fmt.Errorf("erro ao ler resposta: %w", err)
		}
		fmt.Printf("Resposta do servidor: %s\n", resp.Content)
	}

	// também calcula no cliente (round-trip)
	latency := time.Since(start)
	fmt.Printf("Latência medida no cliente: %v\n", latency)

	return nil
}
