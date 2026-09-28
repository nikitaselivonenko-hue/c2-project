// Command c2server — C2-сервер: HTTP для терминала + TCP :445 для клиентов.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"c2project/internal/httpapi"
	"c2project/internal/service"
	"c2project/internal/storage"
	"c2project/internal/tcpserver"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	store := storage.New()
	svc := service.New(store)

	httpSrv := httpapi.NewServer(":8080", httpapi.NewHandler(svc))
	tcpSrv := tcpserver.NewServer(":445", tcpserver.NewClientHandler(svc))

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		log.Println("[C2] HTTP-сервер запущен на :8080")
		return httpSrv.Run(gctx)
	})
	g.Go(func() error {
		return tcpSrv.Run(gctx)
	})

	if err := g.Wait(); err != nil {
		log.Fatalf("[C2] остановлен с ошибкой: %v", err)
	}
	log.Println("[C2] корректно завершён")
}