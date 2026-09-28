package agent

import (
	"errors"
	"os/exec"
	"runtime"
)

// Executor выполняет команды в стандартной оболочке ОС.
type Executor struct{}

// NewExecutor создаёт Executor.
func NewExecutor() *Executor { return &Executor{} }

// Execute выполняет команду, возвращает объединённый вывод STDOUT и STDERR.
// Ненулевой код возврата не считается ошибкой Go: результат всё равно
// возвращается, а текст ошибки добавляется к выводу. Ошибка возвращается
// только если команду невозможно запустить.
func (e *Executor) Execute(cmd string) (string, error) {
	if cmd == "" {
		return "", errors.New("empty command")
	}
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		full := "chcp 866>nul && " + cmd
		out, err = exec.Command("cmd", "/C", full).CombinedOutput()
	} else {
		out, err = exec.Command("sh", "-c", cmd).CombinedOutput()
	}
	result := cp866ToUTF8(string(out))
	if err != nil {
		result += "\n[error] " + err.Error()
	}
	return result, nil
}