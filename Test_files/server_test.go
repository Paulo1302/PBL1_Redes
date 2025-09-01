package server_test

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"PBL1_Redes_Chat_de_texto/server"
	"PBL1_Redes_Chat_de_texto/user"
)

var startOnce sync.Once

// ---------------------- helpers ----------------------

func startServer() {
	startOnce.Do(func() {
		go server.Start()
		time.Sleep(500 * time.Millisecond) // espera o servidor iniciar
	})
}

func readMessages(conn net.Conn, duration time.Duration) []user.Message {
	var messages []user.Message
	reader := bufio.NewReader(conn)
	deadline := time.Now().Add(duration)

	for time.Now().Before(deadline) {
		conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		line, err := reader.ReadString('\n')
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			break
		}
		var msg user.Message
		if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &msg); err == nil {
			messages = append(messages, msg)
		}
	}
	return messages
}

func connectClient(t *testing.T, username string, readTimeout time.Duration) (net.Conn, []user.Message) {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatalf("Erro ao conectar cliente %s: %v", username, err)
	}

	c := user.NewClient(username, conn)
	c.SendMessage(username)

	messages := readMessages(conn, readTimeout)
	return conn, messages
}



// ---------------------- testes ----------------------

func TestClientRequeueOnDisconnect(t *testing.T) {
	startServer()

	// conecta dois clientes
	conn1, _ := connectClient(t, "User1", 100*time.Millisecond)
	defer conn1.Close()
	conn2, _ := connectClient(t, "User2", 100*time.Millisecond)
	defer conn2.Close()

	// aguarda pareamento
	var messages1, messages2 []user.Message
	timeout := time.After(2 * time.Second)
loop:
	for {
		select {
		case <-timeout:
			break loop
		default:
			messages1 = append(messages1, readMessages(conn1, 100*time.Millisecond)...)
			messages2 = append(messages2, readMessages(conn2, 100*time.Millisecond)...)
			gotPair1, gotPair2 := false, false
			for _, m := range messages1 {
				if strings.HasPrefix(m.Content, "Você foi conectado com") {
					gotPair1 = true
				}
			}
			for _, m := range messages2 {
				if strings.HasPrefix(m.Content, "Você foi conectado com") {
					gotPair2 = true
				}
			}
			if gotPair1 && gotPair2 {
				break loop
			}
		}
	}

	t.Logf("Mensagens User1: %+v", messages1)
	t.Logf("Mensagens User2: %+v", messages2)

	// desconecta user2
	conn2.Close()
	time.Sleep(300 * time.Millisecond)

	// conecta user3
	conn3, _ := net.Dial("tcp", "localhost:8080")
	defer conn3.Close()
	c3 := user.NewClient("User3", conn3)
	c3.SendMessage("User3")

	// aguarda requeue e novo pareamento
	messages1 = append(messages1, readMessages(conn1, 1*time.Second)...)
	messages3 := readMessages(conn3, 1*time.Second)

	t.Logf("Mensagens User1 após desconexão: %+v", messages1)
	t.Logf("Mensagens User3: %+v", messages3)

	// verifica se user1 voltou para fila
	requeued := false
	for _, m := range messages1 {
		if strings.HasPrefix(m.Content, "Seu parceiro") && strings.Contains(m.Content, "voltou à fila de espera") {
			requeued = true
			break
		}
	}
	if !requeued {
		t.Errorf("User1 não foi recolocado na fila após desconexão de User2")
	}

	// verifica novo pareamento
	newPair := false
	for _, m := range messages1 {
		if strings.HasPrefix(m.Content, "Você foi conectado com User3") {
			newPair = true
			break
		}
	}
	for _, m := range messages3 {
		if strings.HasPrefix(m.Content, "Você foi conectado com User1") {
			newPair = true
			break
		}
	}
	if !newPair {
		t.Errorf("Novo par User1 <-> User3 não foi formado")
	}
}


func TestStressPairing(t *testing.T) {
	startServer()

	n := 50

	conns := make([]net.Conn, n)
	clients := make([]*user.Client, n)

	// cria clientes e envia registro inicial
	for i := 0; i < n; i++ {
		conn, err := net.Dial("tcp", "localhost:8080")
		if err != nil {
			t.Fatalf("Erro ao conectar cliente %d: %v", i, err)
		}
		conns[i] = conn
		username := "StressUser" + string(rune(i+'A'))
		c := user.NewClient(username, conn)
		clients[i] = c

		// envia registro inicial
		if err := c.SendMessage(username); err != nil {
			t.Fatalf("Erro ao enviar registro para %s: %v", username, err)
		}
	}

	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	// aguarda o servidor processar conexões e formar pares
	time.Sleep(1 * time.Second)

	var wg sync.WaitGroup
	wg.Add(n)

	// cada cliente envia mensagem "Hello"
	for i, c := range clients {
		go func(idx int, cl *user.Client) {
			defer wg.Done()
			if err := cl.SendMessage("Hello"); err != nil {
				t.Errorf("Erro ao enviar mensagem de %s: %v", cl.Name, err)
			}
		}(i, c)
	}

	wg.Wait()

	// verifica se cada cliente recebeu pelo menos a mensagem inicial do servidor
	for i, conn := range conns {
		messages := readMessages(conn, 500*time.Millisecond)
		if len(messages) == 0 {
			t.Errorf("Cliente %s não recebeu nenhuma mensagem do servidor", clients[i].Name)
		}
	}
}