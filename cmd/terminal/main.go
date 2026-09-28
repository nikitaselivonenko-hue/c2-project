// Command terminal — TUI терминала оператора.
package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"

	"c2project/internal/operator"
)

const c2URL = "http://192.168.56.104:8080"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	client := operator.NewClient(c2URL)
	op := operator.New(client)
	if err := op.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("[TERM] завершён с ошибкой: %v", err)
	}
	log.Println("[TERM] корректно завершён")
}