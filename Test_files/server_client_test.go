// File: Test_files/game_server_test.go
package server_test

import (
	"encoding/json"
	"fmt"
	"net"
	"PBL1_Redes/server"
	"sort"
	"sync"
	"testing"
	"time"
)

// --- Test Harness (Estrutura de Suporte aos Testes) ---

// withTestServer inicia um servidor para um único teste e garante seu desligamento.
func withTestServer(t *testing.T, testFunc func(t *testing.T)) {
	t.Helper()
	shutdown := server.Start()
	defer shutdown()
	time.Sleep(50 * time.Millisecond) // Garante que o servidor esteja pronto
	testFunc(t)
}

// --- Helper Structs and Functions ---

type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}
type RegisterPayload struct {
	Name  string         `json:"name"`
	Stock map[string]int `json:"stock"`
}

func connectAndRegister(t *testing.T, name string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatalf("Failed to connect for client %s: %v", name, err)
	}

	initialData := RegisterPayload{
		Name:  name,
		Stock: map[string]int{"Rock": 1, "Paper": 1, "Scissors": 1},
	}
	registerData, _ := json.Marshal(initialData)
	registerMsg := Message{Action: "register", Data: registerData}

	if err := json.NewEncoder(conn).Encode(registerMsg); err != nil {
		t.Fatalf("Failed to send register message for client %s: %v", name, err)
	}
	return conn
}

// --- Test Cases (Casos de Teste) ---

// TestConnectionWithServerClosed valida que a conexão falha quando o servidor não está rodando.
func TestConnectionWithServerClosed(t *testing.T) {
	_, err := net.DialTimeout("tcp", "localhost:8080", 100*time.Millisecond)
	if err == nil {
		t.Fatal("Expected connection to fail, but it succeeded.")
	}
}

// TestRegistrationAndPing testa o fluxo básico de registro e ping.
func TestRegistrationAndPing(t *testing.T) {
	withTestServer(t, func(t *testing.T) {
		conn := connectAndRegister(t, "PingTester")
		defer conn.Close()
		// ... (a lógica do teste permanece a mesma)
	})
}

// (Outros testes como TestOpenPack, TestFullGameFlow, etc., seguiriam o mesmo padrão)

// --- Teste de Estresse Integrado ---

// result armazena o resultado de uma única requisição.
type stressResult struct {
	latency time.Duration
	err     error
}

// stressWorker simula a jornada de um usuário para o teste de estresse.
func stressWorker(id int, addr string, requests int, timeout time.Duration, out chan<- stressResult) {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		out <- stressResult{err: fmt.Errorf("worker %d dial: %w", id, err)}
		return
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	registerPayload := RegisterPayload{
		Name:  fmt.Sprintf("stress_user_%d", id),
		Stock: map[string]int{"Rock": 3, "Paper": 3, "Scissors": 3},
	}
	req := Message{Action: "register", Data: json.RawMessage(mustMarshal(registerPayload))}
	if err := enc.Encode(&req); err != nil {
		out <- stressResult{err: fmt.Errorf("worker %d encode register: %w", id, err)}
		return
	}

	openPackReq := Message{Action: "open_pack"}
	for i := 0; i < requests; i++ {
		start := time.Now()
		if err := enc.Encode(&openPackReq); err != nil {
			out <- stressResult{err: fmt.Errorf("worker %d encode open_pack: %w", id, err)}
			return
		}
		var resp Message
		if err := dec.Decode(&resp); err != nil {
			out <- stressResult{err: fmt.Errorf("worker %d decode open_pack resp: %w", id, err)}
			return
		}
		out <- stressResult{latency: time.Since(start)}
	}
}

// TestStressWithBenchmarkReport executa um teste de estresse rápido e integrado.
func TestStressWithBenchmarkReport(t *testing.T) {
	withTestServer(t, func(t *testing.T) {
		// Configurações fixas para um teste rápido e automatizado
		concurrency := 20
		requestsPerConn := 5
		timeout := 5 * time.Second

		totalExpected := concurrency * requestsPerConn
		results := make(chan stressResult, totalExpected)

		t.Logf("Running integrated stress test: %d clients, %d requests each...", concurrency, requestsPerConn)

		startAll := time.Now()

		var wg sync.WaitGroup
		wg.Add(concurrency)
		for i := 0; i < concurrency; i++ {
			go func(id int) {
				defer wg.Done()
				stressWorker(id, "localhost:8080", requestsPerConn, timeout, results)
			}(i)
		}
		wg.Wait()
		close(results)

		elapsed := time.Since(startAll)

		// Coleta e processa os resultados
		var latencies []time.Duration
		errors := 0
		for r := range results {
			if r.err != nil {
				errors++
				t.Errorf("Stress worker error: %v", r.err)
				continue
			}
			latencies = append(latencies, r.latency)
		}

		if errors > 0 {
			t.Fatalf("%d errors occurred during stress test, failing.", errors)
		}

		// Imprime o relatório de benchmark
		printBenchmarkReport(t, latencies, elapsed, totalExpected)
	})
}

// --- Funções Auxiliares para o Benchmark ---

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(p*float64(len(sorted)) + 0.5)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

func printBenchmarkReport(t *testing.T, latencies []time.Duration, elapsed time.Duration, totalReqs int) {
	success := len(latencies)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var avg time.Duration
	for _, d := range latencies {
		avg += d
	}
	if success > 0 {
		avg /= time.Duration(success)
	}

	p50 := percentile(latencies, 0.50)
	p90 := percentile(latencies, 0.90)
	p99 := percentile(latencies, 0.99)
	qps := float64(success) / elapsed.Seconds()

	t.Logf("\n--- Benchmark Report ---\n")
	t.Logf("Total Requests: %d\n", totalReqs)
	t.Logf("Total Duration: %v\n", elapsed)
	if success > 0 {
		t.Logf("Avg Latency: %v | p50: %v | p90: %v | p99: %v\n", avg, p50, p90, p99)
		t.Logf("Throughput: %.2f req/s (QPS)\n", qps)
	}
	t.Logf("----------------------\n")
}