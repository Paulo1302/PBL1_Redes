package user

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

// Message representa uma mensagem enviada entre cliente e servidor
type Message struct {
	Sender   string `json:"user"`    // remetente
	Receiver string `json:"to"`      // destinatário
	Content  string `json:"content"` // conteúdo da mensagem
}

// Client representa o cliente conectado
type Client struct {
	Name                 string
	Connection           net.Conn
	IsConnectedWithPartner bool
}

// NewClient cria um novo cliente
func NewClient(name string, conn net.Conn) *Client {
	return &Client{
		Name:       name,
		Connection: conn,
	}
}

// StartListener inicia a goroutine para ouvir mensagens do servidor
func (c *Client) StartListener() {
	go func() {
		reader := bufio.NewReader(c.Connection)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("\n❌ Conexão encerrada pelo servidor.")
				os.Exit(0)
			}

			var incoming Message
			if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &incoming); err == nil {
				if incoming.Sender == "Servidor" {
					fmt.Printf("\n📩 [Servidor]: %s\n", incoming.Content)

					if strings.HasPrefix(incoming.Content, "Você foi conectado com") {
						c.IsConnectedWithPartner = true
					}

					if strings.Contains(incoming.Content, "voltou à fila de espera") {
						c.IsConnectedWithPartner = false
					}
				} else {
					fmt.Printf("\n📩 [%s]: %s\n", incoming.Sender, incoming.Content)
				}
			}
		}
	}()
}

// SendMessage envia uma mensagem ao servidor
func (c *Client) SendMessage(content string) error {
	msg := Message{
		Sender:  c.Name,
		Content: content,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = c.Connection.Write(append(data, '\n'))
	return err
}
