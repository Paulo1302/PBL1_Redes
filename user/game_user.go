// File: user/game_user.go
package user

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// Message define a estrutura padrão para a troca de dados com o servidor.
// É idêntica à do servidor para garantir compatibilidade.
type Message struct {
	Action string          `json:"action"` // Ação a ser executada (ex: "ping", "play").
	Data   json.RawMessage `json:"data"`   // Dados associados à ação.
}

// UserData armazena o estado do jogador que será salvo e carregado localmente
// no arquivo 'user_data.json'.
type UserData struct {
	Name  string         `json:"name"`
	Stock map[string]int `json:"stock"`
}

// Payloads para decodificar o campo 'Data' das mensagens recebidas do servidor.
// Cada struct corresponde a uma ação específica.
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
type OpenPackData struct {
	Content  string         `json:"content"`
	NewCards []string       `json:"new_cards"`
	NewStock map[string]int `json:"new_stock"`
}
type RoundResultData struct {
	Result     string `json:"result"`
	Winner     string `json:"winner,omitempty"`
	Loser      string `json:"loser,omitempty"`
	WinnerCard string `json:"winner_card,omitempty"`
	LoserCard  string `json:"loser_card,omitempty"`
	DrawCard   string `json:"draw_card,omitempty"`
}

// PingManager gerencia o estado do ping para calcular a latência de forma segura
// entre a goroutine principal e a 'listenServer'.
type PingManager struct {
	mu        sync.Mutex
	startTime time.Time
}

// Variáveis globais para manter o estado do cliente.
var (
	currentUserData UserData    // Armazena os dados do usuário carregados/criados.
	pingManager     = &PingManager{} // Gerenciador de estado do ping.
)

const userDataFile = "user_data.json" // Nome do arquivo de salvamento local.

// saveData serializa a struct 'currentUserData' para o arquivo JSON, persistindo o progresso.
func saveData() error {
	file, err := os.Create(userDataFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Formata o JSON para ser legível.
	return encoder.Encode(currentUserData)
}

// loadData carrega os dados do usuário do arquivo JSON. Se o arquivo não existir,
// ela inicia o processo de criação de um novo perfil.
func loadData() {
	file, err := os.Open(userDataFile)
	// Se o arquivo não puder ser aberto, assume-se que é um novo jogador.
	if err != nil {
		fmt.Println("Nenhum arquivo de usuário encontrado. Criando um novo perfil.")
		fmt.Print("Digite seu nome: ")
		reader := bufio.NewReader(os.Stdin)
		name, _ := reader.ReadString('\n')

		// Cria a estrutura de dados inicial para o novo usuário.
		currentUserData = UserData{
			Name: strings.TrimSpace(name),
			Stock: map[string]int{ // O inventário inicial é zero.
				"Rock":     0,
				"Paper":    0,
				"Scissors": 0,
			},
		}
		if err := saveData(); err != nil {
			fmt.Printf("Erro ao salvar novo perfil: %v\n", err)
		}
		return
	}
	defer file.Close()

	// Se o arquivo existe, decodifica o JSON para a struct 'currentUserData'.
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&currentUserData); err != nil {
		fmt.Printf("Erro ao ler arquivo de perfil: %v\n", err)
	}
	fmt.Printf("Bem-vindo de volta, %s!\n", currentUserData.Name)
}

// ServerConn é a função principal do cliente. Ela estabelece a conexão, gerencia
// o loop de entrada de comandos do usuário e a comunicação com o servidor.
func ServerConn(serverIP string) error {
	// Carrega ou cria os dados do usuário antes de qualquer outra coisa.
	loadData()

	// Tenta estabelecer a conexão TCP com o servidor.
	conn, err := net.Dial("tcp", serverIP+":8080")
	if err != nil {
		return fmt.Errorf("erro ao conectar no servidor: %w", err)
	}
	defer conn.Close()
	fmt.Println("Conectado ao servidor. Digite 'help' para ver os comandos.")

	encoder := json.NewEncoder(conn)

	// A primeira ação após conectar é se registrar no servidor.
	registerData, _ := json.Marshal(currentUserData)
	registerMsg := Message{Action: "register", Data: registerData}
	if err := encoder.Encode(registerMsg); err != nil {
		return fmt.Errorf("erro ao registrar no servidor: %w", err)
	}

	// Inicia uma goroutine para ouvir mensagens do servidor de forma assíncrona.
	go listenServer(conn)

	// Loop principal para ler comandos do terminal.
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		parts := strings.Split(input, " ")
		command := parts[0]

		var msg Message
		var data interface{}

		// Switch para interpretar o comando do usuário e preparar a mensagem para o servidor.
		switch command {
		case "ping":
			pingManager.mu.Lock()
			pingManager.startTime = time.Now() // Marca o tempo de início do ping.
			pingManager.mu.Unlock()
			msg.Action = "ping"
		case "play":
			// Validação do lado do cliente para dar feedback instantâneo.
			totalCards := 0
			for _, quantity := range currentUserData.Stock {
				totalCards += quantity
			}

			if totalCards == 0 {
				fmt.Println("Você não tem cartas para jogar! Use o comando 'pack' para conseguir mais.")
				continue // Impede o envio da mensagem e aguarda o próximo comando.
			}
			
			msg.Action = "queue_for_match"
		case "pack":
			msg.Action = "open_pack"
		case "move":
			if len(parts) < 2 {
				fmt.Println("Uso: move <Rock|Paper|Scissors>")
				continue
			}
			msg.Action = "play" // No servidor, 'move' é tratado pela ação 'play'.
			data = PlayData{ChosenCard: parts[1]}
		case "stock":
			// Comando puramente local, não envia mensagem ao servidor.
			fmt.Println("Seu inventário de cartas:")
			if len(currentUserData.Stock) == 0 {
				fmt.Println("  - Nenhuma carta restante.")
			} else {
				totalCards := 0
				for card, quantity := range currentUserData.Stock {
					fmt.Printf("  - %s: %d\n", card, quantity)
					totalCards += quantity
				}
				if totalCards == 0 {
					fmt.Println("  (Você precisa abrir um pacote para obter novas cartas)")
				}
			}
			continue
		case "help":
			// Comando puramente local.
			fmt.Println("\nComandos disponíveis:")
			fmt.Println("  stock         - Mostra as cartas que você possui.")
			fmt.Println("  pack          - Abre um pacote de 3 cartas aleatórias.")
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
			if command != "" {
				fmt.Printf("Comando desconhecido: '%s'. Digite 'help'.\n", command)
			}
			continue
		}

		// Se o comando gerou dados (como em 'move'), codifica-os para JSON.
		if data != nil {
			jsonData, err := json.Marshal(data)
			if err != nil {
				fmt.Printf("Erro ao codificar dados: %v\n", err)
				continue
			}
			msg.Data = jsonData
		}

		// Envia a mensagem final para o servidor.
		if err := encoder.Encode(msg); err != nil {
			fmt.Printf("Erro ao enviar comando para o servidor: %v\n", err)
			return err
		}
	}
	return scanner.Err()
}

// listenServer processa mensagens recebidas do servidor em uma goroutine separada.
// Isso permite que o usuário continue digitando comandos enquanto espera por respostas.
func listenServer(conn net.Conn) {
	decoder := json.NewDecoder(conn)
	for {
		var msg Message
		// Bloqueia até que uma nova mensagem seja recebida do servidor.
		if err := decoder.Decode(&msg); err != nil {
			// Se houver um erro, geralmente significa que a conexão foi perdida.
			fmt.Printf("\n[AVISO] Conexão com o servidor perdida: %v\n", err)
			os.Exit(1) // Encerra o programa cliente.
			return
		}

		// Imprime um cabeçalho para indicar que a mensagem veio do servidor.
		fmt.Printf("\n<-- [MENSAGEM DO SERVIDOR | Ação: %s]\n", msg.Action)

		// Switch para tratar diferentes ações recebidas do servidor.
		switch msg.Action {
		case "pong":
			pingManager.mu.Lock()
			if !pingManager.startTime.IsZero() {
				latency := time.Since(pingManager.startTime)
				fmt.Printf("    Latência (RTT): %v\n", latency)
				pingManager.startTime = time.Time{} // Reseta o tempo de início.
			}
			pingManager.mu.Unlock()
		case "game_over":
			var gameOverData GameOverData
			if err := json.Unmarshal(msg.Data, &gameOverData); err == nil {
				fmt.Printf("    Conteúdo: %s\n", gameOverData.Content)
				// Atualiza o inventário local com o resultado final da partida.
				if gameOverData.FinalStock != nil {
					currentUserData.Stock = gameOverData.FinalStock
					if err := saveData(); err != nil {
						fmt.Printf("    ERRO: Falha ao salvar seu progresso: %v\n", err)
					} else {
						fmt.Printf("    Progresso salvo! Novo inventário: %v\n", currentUserData.Stock)
					}
				}
			}
		case "open_pack_success":
			var packData OpenPackData
			if err := json.Unmarshal(msg.Data, &packData); err == nil {
				fmt.Printf("    %s\n", packData.Content)
				fmt.Printf("    Você recebeu: %v\n", packData.NewCards)
				// Atualiza o inventário local com as novas cartas.
				currentUserData.Stock = packData.NewStock
				if err := saveData(); err != nil {
					fmt.Printf("    ERRO: Falha ao salvar seu novo inventário: %v\n", err)
				} else {
					fmt.Printf("    Inventário salvo! %v\n", currentUserData.Stock)
				}
			}
		case "game_round_result":
			// Nota: Este caso pode não ser mais usado se o jogo for sempre de rodada única.
			var roundData RoundResultData
			if err := json.Unmarshal(msg.Data, &roundData); err == nil {
				if roundData.Result == "Draw" {
					fmt.Printf("    Resultado: Empate! Ambos jogaram %s.\n", roundData.DrawCard)
				} else {
					fmt.Printf("    Resultado: %s jogou %s e venceu %s, que jogou %s.\n", roundData.Winner, roundData.WinnerCard, roundData.Loser, roundData.LoserCard)
				}
			}
		default: // Caso padrão para mensagens informativas ou de erro.
			var response ResponseData
			_ = json.Unmarshal(msg.Data, &response)
			if response.Error != "" {
				fmt.Printf("    Erro: %s\n", response.Error)
			} else {
				fmt.Printf("    Conteúdo: %s\n", response.Content)
			}
		}
		// Exibe novamente o prompt para o usuário após processar a mensagem do servidor.
		fmt.Print("--> Digite seu comando: ")
	}
}