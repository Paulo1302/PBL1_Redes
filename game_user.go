// File: user/game_user.go
package user

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

// Message usa o mesmo formato do servidor para compatibilidade.
type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// UserData armazena o estado do jogador que será salvo localmente.
type UserData struct {
	Name  string         `json:"name"`
	Stock map[string]int `json:"stock"`
}

// Payloads para diferentes ações
type ResponseData struct {
	Content string `json:"content"`
	Error   string `json:"error"`
}
type PlayData struct {
	ChosenCard string `json:"chosen_card"`
}
type GameOverData struct {
	Content    string         `json:"content"`
	FinalStock map[string]int `json:"final_stock"`
}
// OpenPackData para decodificar a resposta de abrir pacote
type OpenPackData struct {
	Content  string         `json:"content"`
	NewCards []string       `json:"new_cards"`
	NewStock map[string]int `json:"new_stock"`
}

var currentUserData UserData
const userDataFile = "user_data.json"

func saveData() error {
	file, err := os.Create(userDataFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(currentUserData)
}

func loadData() {
	file, err := os.Open(userDataFile)
	if err != nil {
		fmt.Println("Nenhum arquivo de usuário encontrado. Criando um novo perfil.")
		fmt.Print("Digite seu nome: ")
		reader := bufio.NewReader(os.Stdin)
		name, _ := reader.ReadString('\n')
		
		currentUserData = UserData{
			Name: strings.TrimSpace(name),
			Stock: map[string]int{
				"Rock":     3,
				"Paper":    3,
				"Scissors": 3,
			},
		}
		if err := saveData(); err != nil {
			fmt.Printf("Erro ao salvar novo perfil: %v\n", err)
		}
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&currentUserData); err != nil {
		fmt.Printf("Erro ao ler arquivo de perfil: %v\n", err)
	}
	fmt.Printf("Bem-vindo de volta, %s!\n", currentUserData.Name)
}

func ServerConn(serverIP string) error {
	loadData()

	// Usando o IP recebido como argumento para a conexão
	conn, err := net.Dial("tcp", serverIP+":8080") // Porta permanece fixa em 8080
	if err != nil {
		return fmt.Errorf("erro ao conectar no servidor: %w", err)
	}
	defer conn.Close()
	fmt.Println("Conectado ao servidor. Digite 'help' para ver os comandos.")

	encoder := json.NewEncoder(conn)

	registerData, _ := json.Marshal(currentUserData)
	registerMsg := Message{Action: "register", Data: registerData}
	if err := encoder.Encode(registerMsg); err != nil {
		return fmt.Errorf("erro ao registrar no servidor: %w", err)
	}

	go listenServer(conn)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		// O código continua igual, sem mudanças aqui
		input := strings.TrimSpace(scanner.Text())
		parts := strings.Split(input, " ")
		command := parts[0]

		var msg Message
		var data interface{}

		switch command {
		case "ping":
			msg.Action = "ping"
		case "play":
			msg.Action = "queue_for_match"
		case "pack": // NOVO COMANDO
			msg.Action = "open_pack"
		case "move":
			if len(parts) < 2 {
				fmt.Println("Uso: move <Rock|Paper|Scissors>")
				continue
			}
			msg.Action = "play"
			data = PlayData{ChosenCard: parts[1]}
		case "stock":
			fmt.Println("Seu inventário de cartas:")
			if len(currentUserData.Stock) == 0 {
				fmt.Println("  - Nenhuma carta restante.")
			} else {
				for card, quantity := range currentUserData.Stock {
					fmt.Printf("  - %s: %d\n", card, quantity)
				}
			}
			continue
		case "help":
			fmt.Println("\nComandos disponíveis:")
			fmt.Println("  stock         - Mostra as cartas que você possui.")
			fmt.Println("  pack          - Abre um pacote de 3 cartas aleatórias.") // AJUDA ATUALIZADA
			fmt.Println("  ping          - Verifica a latência com o servidor.")
			fmt.Println("  play          - Entra na fila para encontrar uma partida.")
			fmt.Println("  move <card>   - Joga uma carta (ex: move Rock).")
			fmt.Println("  exit          - Fecha a conexão.")
			fmt.Println()
			continue
		case "exit":
			fmt.Println("Desconectando...")
			return nil
		default:
			fmt.Printf("Comando desconhecido: '%s'. Digite 'help'.\n", command)
			continue
		}
		
		if data != nil {
			jsonData, err := json.Marshal(data)
			if err != nil {
				fmt.Printf("Erro ao codificar dados: %v\n", err)
				continue
			}
			msg.Data = jsonData
		}

		if err := encoder.Encode(msg); err != nil {
			fmt.Printf("Erro ao enviar comando para o servidor: %v\n", err)
			return err
		}
	}
	return scanner.Err()
}
func listenServer(conn net.Conn) {
	decoder := json.NewDecoder(conn)
	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			fmt.Printf("\n[AVISO] Conexão com o servidor perdida: %v\n", err)
			os.Exit(1)
			return
		}

		fmt.Printf("\n<-- [MENSAGEM DO SERVIDOR | Ação: %s]\n", msg.Action)

		// O 'switch' ajuda a organizar o tratamento de diferentes ações
		switch msg.Action {
		case "game_over":
			var gameOverData GameOverData
			if err := json.Unmarshal(msg.Data, &gameOverData); err == nil {
				fmt.Printf("    Conteúdo: %s\n", gameOverData.Content)
				if gameOverData.FinalStock != nil {
					currentUserData.Stock = gameOverData.FinalStock
					if err := saveData(); err != nil {
						fmt.Printf("    ERRO: Falha ao salvar seu progresso: %v\n", err)
					} else {
						fmt.Printf("    Progresso salvo! Novo inventário: %v\n", currentUserData.Stock)
					}
				}
			}
		case "open_pack_success": // NOVO CASE
			var packData OpenPackData
			if err := json.Unmarshal(msg.Data, &packData); err == nil {
				fmt.Printf("    %s\n", packData.Content)
				fmt.Printf("    Você recebeu: %v\n", packData.NewCards)
				
				currentUserData.Stock = packData.NewStock // Atualiza o inventário com a versão do servidor
				if err := saveData(); err != nil {
					fmt.Printf("    ERRO: Falha ao salvar seu novo inventário: %v\n", err)
				} else {
					fmt.Printf("    Inventário salvo! %v\n", currentUserData.Stock)
				}
			}
		default: // Mensagens normais
			var response ResponseData
			_ = json.Unmarshal(msg.Data, &response)
			if response.Error != "" {
				fmt.Printf("    Erro: %s\n", response.Error)
			} else {
				fmt.Printf("    Conteúdo: %s\n", response.Content)
			}
		}
		fmt.Print("--> Digite seu comando: ")
	}
}