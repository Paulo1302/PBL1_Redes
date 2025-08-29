package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"PBL1_Redes_Chat_de_texto/user" // ajuste para o nome correto do seu módulo
)

func main() {
	// Solicita nome do usuário
	fmt.Print("Digite seu nome de usuário: ")
	consoleReader := bufio.NewReader(os.Stdin)
	userNameInput, _ := consoleReader.ReadString('\n')
	userName := strings.TrimSpace(userNameInput)

	// Conecta ao servidor
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Erro ao conectar ao servidor:", err)
		return
	}
	defer conn.Close()

	// Cria o cliente
	c := user.NewClient(userName, conn)

	// Envia mensagem inicial de registro
	if err := c.SendMessage(userName); err != nil {
		fmt.Println("Erro ao enviar mensagem inicial:", err)
		return
	}

	// Inicia listener de mensagens
	c.StartListener()

	// Loop principal para envio de mensagens
	for {
		fmt.Print("\nDigite sua mensagem (ou 'exit' para sair): ")
		text, _ := consoleReader.ReadString('\n')
		message := strings.TrimSpace(text)

		if message == "exit" {
			fmt.Println("Conexão encerrada pelo usuário.")
			break
		}

		if !c.IsConnectedWithPartner {
			fmt.Println("⚠️  Você ainda não está conectado a um parceiro. Aguarde...")
			continue
		}

		if err := c.SendMessage(message); err != nil {
			fmt.Println("Erro ao enviar mensagem:", err)
		}
	}
}
