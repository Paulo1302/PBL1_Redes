package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

// Estrutura da mensagem (igual à do servidor)
type Message struct {
	User    string `json:"user"`    // remetente
	To      string `json:"to"`      // destinatário
	Content string `json:"content"` // conteúdo
}

func main() {
	// Pergunta o nome do usuário
	fmt.Print("Digite seu nome de usuário: ")
	inputReader := bufio.NewReader(os.Stdin)
	userName, _ := inputReader.ReadString('\n')
	userName = strings.TrimSpace(userName)

	// Conectar ao servidor
	serverConnection, err := net.Dial("tcp", ":8080")
	if err != nil {
		fmt.Println("Erro ao conectar ao servidor:", err)
		return
	}
	defer serverConnection.Close()

	// Envia mensagem inicial para registrar no servidor
	initialMessage := Message{User: userName}
	data, _ := json.Marshal(initialMessage)
	serverConnection.Write(data)

	// Variável para saber se está conectado a um parceiro
	isPaired := false

	// Goroutine para ouvir mensagens do servidor
	go func() {
		for {
			readBuffer := make([]byte, 1024)
			n, err := serverConnection.Read(readBuffer)
			if err != nil {
				fmt.Println("\n❌ Conexão encerrada pelo servidor.")
				os.Exit(0)
			}

			var receivedMessage Message
			err = json.Unmarshal(readBuffer[:n], &receivedMessage)
			if err == nil {
				// Mensagem de status do servidor
				if receivedMessage.User == "Servidor" {
					fmt.Printf("\n📩 [Servidor]: %s\n", receivedMessage.Content)
					if strings.HasPrefix(receivedMessage.Content, "Você foi conectado com") {
						isPaired = true
					}
				} else {
					// Mensagem do parceiro
					fmt.Printf("\n📩 [%s]: %s\n", receivedMessage.User, receivedMessage.Content)
				}
			}
		}
	}()

	// Loop principal para enviar mensagens
	for {
		fmt.Print("\nDigite sua mensagem (ou 'exit' para sair): ")
		userInput, _ := inputReader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)

		if userInput == "exit" {
			fmt.Println("Conexão encerrada.")
			break
		}

		if !isPaired {
			fmt.Println("⚠️  Você ainda não está conectado a um parceiro. Aguarde...")
			continue
		}

		// Cria a mensagem (não precisa informar "To", o servidor sabe)
		outgoingMessage := Message{
			User:    userName,
			Content: userInput,
		}

		// Serializa e envia
		data, err := json.Marshal(outgoingMessage)
		if err != nil {
			fmt.Println("Erro ao serializar mensagem:", err)
			continue
		}
		serverConnection.Write(data)
	}
}
