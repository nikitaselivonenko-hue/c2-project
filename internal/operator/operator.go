// Package operator реализует TUI терминала оператора.
package operator

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// Operator — TUI оператора.
type Operator struct {
	client *Client
	reader *bufio.Reader
}

// New создаёт оператора.
func New(client *Client) *Operator {
	return &Operator{
		client: client,
		reader: bufio.NewReader(os.Stdin),
	}
}

// Run — главный цикл терминала.
func (o *Operator) Run(ctx context.Context) error {
	setConsoleUTF8()
	fmt.Println("=== C2 Terminal ===")
	var clientID string
	for {
		fmt.Println("\nКоманды: /clients — выбрать клиента, /exit — выход")
		fmt.Print("> ")
		line, err := o.reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read input: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch line {
		case "/exit":
			return nil
		case "/clients":
			clientID = o.pickClient(ctx)
			continue
		}
		if clientID == "" {
			fmt.Println("[!] Сначала выберите клиента через /clients")
			continue
		}
		if err := o.runCommand(ctx, clientID, line); err != nil {
			fmt.Printf("[!] %v\n", err)
		}
	}
}

// pickClient выводит список клиентов и предлагает выбрать номер.
func (o *Operator) pickClient(ctx context.Context) string {
	clients, err := o.client.ListClients(ctx)
	if err != nil {
		fmt.Printf("[!] Ошибка получения списка клиентов: %v\n", err)
		return ""
	}
	if len(clients) == 0 {
		fmt.Println("[!] Нет зарегистрированных клиентов")
		return ""
	}
	fmt.Println("Доступные клиенты:")
	for i, c := range clients {
		fmt.Printf("  %d) %s | %s | %s | last seen: %s\n", i+1, c.ClientID, c.Hostname, c.OS, c.LastSeen)
	}
	fmt.Print("Выберите номер клиента: ")
	line, _ := o.reader.ReadString('\n')
	var idx int
	if _, err := fmt.Sscanf(strings.TrimSpace(line), "%d", &idx); err != nil {
		fmt.Println("[!] Неверный номер")
		return ""
	}
	if idx < 1 || idx > len(clients) {
		fmt.Println("[!] Неверный номер")
		return ""
	}
	return clients[idx-1].ClientID
}

// runCommand отправляет команду и ждёт результат.
func (o *Operator) runCommand(ctx context.Context, clientID, command string) error {
	taskID, err := o.client.Submit(ctx, clientID, command)
	if err != nil {
		return fmt.Errorf("submit: %w", err)
	}
	fmt.Printf("[*] Задача %s отправлена, ожидание результата...\n", taskID)

	result, err := o.client.WaitResult(ctx, taskID)
	if err != nil {
		return fmt.Errorf("wait result: %w", err)
	}
	fmt.Println("--- Результат ---")
	fmt.Println(result)
	fmt.Println("-----------------")
	return nil
}