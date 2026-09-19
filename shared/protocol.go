// Файл содержит константы и типы протокола верхнего уровня:
// ключи шифрования, эндпоинты HTTP API и JSON-структуры обмена
// между терминалом оператора и C2-сервером.
package shared

// Ключи симметричного шифрования AES-256-GCM (длина каждого — 32 байта).
// В учебной реализации ключи зашиты в код. В реальной системе их
// следовало бы получать через процедуру обмена ключами (например,
// Диффи-Хеллмана) и хранить в защищённом хранилище.
var (
	KeyTerminalToC2 = []byte("0123456789abcdef0123456789abcdef") // ключ для канала терминал ↔ C2
	KeyC2ToClient   = []byte("fedcba9876543210fedcba9876543210") // ключ для канала C2 ↔ клиент
)

// Эндпоинты HTTP API, обслуживаемые C2-сервером для терминала оператора.
const (
	EndpointSubmit = "/api/submit"  // POST: отправить команду на исполнение
	EndpointStatus = "/api/status"  // GET: узнать статус задачи по её ID
	EndpointList   = "/api/clients" // GET: получить список зарегистрированных клиентов

	// Возможные статусы задачи в хранилище C2.
	StatusPending = "pending" // задача ожидает исполнения
	StatusDone    = "done"    // задача выполнена, есть результат
	StatusError   = "error"   // при выполнении задачи произошла ошибка
)

// SubmitRequest — тело POST-запроса от терминала на C2.
// Поле Command содержит base64-представление зашифрованной команды.
type SubmitRequest struct {
	ClientID string `json:"client_id"` // идентификатор целевого клиента
	Command  string `json:"command"`   // зашифрованная команда в base64
}

// SubmitResponse — ответ C2 терминалу после постановки задачи.
// Содержит присвоенный задаче идентификатор.
type SubmitResponse struct {
	TaskID string `json:"task_id"` // идентификатор созданной задачи
}

// StatusResponse — ответ C2 на запрос статуса задачи.
// Если статус StatusDone, поле Result содержит base64 зашифрованного
// результата выполнения, предназначенного для расшифровки терминалом.
type StatusResponse struct {
	TaskID string `json:"task_id"`         // идентификатор задачи
	Status string `json:"status"`          // текущий статус (pending, done, error)
	Result string `json:"result,omitempty"` // зашифрованный результат в base64
}

// ClientInfo — информация о зарегистрированном клиенте, отдаваемая
// терминалу в ответ на запрос EndpointList.
type ClientInfo struct {
	ClientID string `json:"client_id"` // уникальный идентификатор клиента
	Hostname string `json:"hostname"`  // имя хоста
	OS       string `json:"os"`        // название операционной системы
	LastSeen string `json:"last_seen"` // время последнего обращения (RFC3339)
}
