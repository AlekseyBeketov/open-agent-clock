package interactive

import "fmt"

type message struct {
	EN string
	RU string
}

var messages = map[string]message{
	"language.title":       {"Choose your language / Выберите язык", "Choose your language / Выберите язык"},
	"language.description": {"English is selected by default. You can change this later in Settings.", "По умолчанию выбран English. Язык можно изменить позже в настройках."},
	"language.english":     {"English", "English"},
	"language.russian":     {"Русский", "Русский"},

	"common.yes":       {"Yes", "Да"},
	"common.no":        {"No", "Нет"},
	"common.back":      {"Back", "Назад"},
	"common.exit":      {"Exit", "Выйти"},
	"common.continue":  {"Continue", "Продолжить"},
	"common.retry":     {"Check again", "Проверить снова"},
	"common.cancelled": {"Cancelled. Nothing unsafe was started.", "Отменено. Ничего небезопасного не запущено."},

	"welcome.title":  {"Welcome to open-agent-clock", "Добро пожаловать в open-agent-clock"},
	"welcome.body":   {"This guided setup configures official local agent CLIs. It is not a limit bypass or resetter, and a scheduled call may consume subscription allowance.", "Мастер настроит официальные локальные agent CLI. Это не обходчик и не сбросчик лимитов; запланированный вызов может расходовать subscription allowance."},
	"welcome.accept": {"I understand. Start setup?", "Понятно. Начать настройку?"},

	"setup.target":            {"Choose an available subscription target", "Выберите доступный subscription target"},
	"setup.no_targets":        {"No available subscription target was found. Install and authenticate an official CLI, then check again. open-agent-clock never asks for credentials.", "Доступные subscription targets не найдены. Установите и авторизуйте официальный CLI, затем повторите проверку. open-agent-clock никогда не запрашивает credentials."},
	"setup.no_targets_action": {"What would you like to do?", "Что сделать?"},
	"setup.id":                {"Schedule ID", "ID расписания"},
	"setup.id_help":           {"A short stable name, for example codex-window", "Короткое постоянное имя, например codex-window"},
	"setup.name":              {"Display name", "Отображаемое имя"},
	"setup.mode":              {"Schedule mode", "Режим расписания"},
	"setup.mode.interval":     {"Recurring interval", "Повторяющийся интервал"},
	"setup.mode.morning":      {"Every morning", "Каждое утро"},
	"setup.mode.daily":        {"Custom daily times", "Заданные часы каждый день"},
	"setup.interval":          {"Interval", "Интервал"},
	"setup.time":              {"Local time (HH:MM)", "Локальное время (HH:MM)"},
	"setup.times":             {"Daily times, comma-separated", "Время запусков через запятую"},
	"setup.timezone":          {"IANA timezone", "Часовой пояс IANA"},
	"setup.prompt":            {"Provider prompt", "Prompt для провайдера"},
	"setup.review.title":      {"Review before saving", "Проверка перед сохранением"},
	"setup.review.body":       {"Review these values. No schedule will be saved until you confirm.", "Проверьте значения. До подтверждения расписание не будет сохранено."},
	"setup.review.id":         {"ID", "ID"},
	"setup.review.target":     {"Target", "Target"},
	"setup.review.mode":       {"Mode", "Режим"},
	"setup.review.timing":     {"Timing", "Время"},
	"setup.review.timezone":   {"Timezone", "Часовой пояс"},
	"setup.review.prompt":     {"Prompt", "Prompt"},
	"setup.summary":           {"Review the schedule", "Проверьте расписание"},
	"setup.preview":           {"Show the exact provider command without running it?", "Показать точную provider command без запуска?"},
	"setup.activate":          {"Enable this schedule? Future runs may consume subscription allowance.", "Включить расписание? Будущие запуски могут расходовать subscription allowance."},
	"setup.install":           {"Install the user-level macOS LaunchAgent? Future runs may consume subscription allowance.", "Установить user-level macOS LaunchAgent? Будущие запуски могут расходовать subscription allowance."},
	"setup.complete":          {"Setup complete. You can reopen this menu with `open-agent-clock` or `open-agent-clock ui`.", "Настройка завершена. Открыть меню снова можно командами `open-agent-clock` или `open-agent-clock ui`."},

	"main.title":       {"open-agent-clock", "open-agent-clock"},
	"main.description": {"Choose what you want to do", "Выберите действие"},
	"main.dashboard":   {"Dashboard", "Обзор"},
	"main.targets":     {"Targets", "Targets"},
	"main.schedules":   {"Schedules", "Расписания"},
	"main.run":         {"Run or preview", "Запуск или preview"},
	"main.history":     {"History", "История"},
	"main.settings":    {"Settings", "Настройки"},
	"main.setup":       {"Run guided setup", "Запустить мастер настройки"},

	"schedule.title":         {"Schedule management", "Управление расписаниями"},
	"schedule.list":          {"List schedules", "Список расписаний"},
	"schedule.inspect":       {"Inspect schedule", "Посмотреть расписание"},
	"schedule.create":        {"Create schedule", "Создать расписание"},
	"schedule.edit":          {"Edit schedule", "Изменить расписание"},
	"schedule.toggle":        {"Enable or pause schedule", "Включить или приостановить"},
	"schedule.launchd":       {"LaunchAgent management", "Управление LaunchAgent"},
	"schedule.remove":        {"Remove schedule", "Удалить расписание"},
	"schedule.select":        {"Choose a schedule", "Выберите расписание"},
	"schedule.empty":         {"No schedules exist yet. Create one first.", "Расписаний пока нет. Сначала создайте расписание."},
	"schedule.enable":        {"Enable", "Включить"},
	"schedule.pause":         {"Pause", "Приостановить"},
	"schedule.state.enabled": {"enabled", "включено"},
	"schedule.state.paused":  {"paused", "приостановлено"},

	"launchd.title":     {"LaunchAgent management", "Управление LaunchAgent"},
	"launchd.preview":   {"Preview plist", "Preview plist"},
	"launchd.install":   {"Install LaunchAgent", "Установить LaunchAgent"},
	"launchd.status":    {"Show LaunchAgent status", "Показать статус LaunchAgent"},
	"launchd.uninstall": {"Uninstall LaunchAgent", "Удалить LaunchAgent"},

	"run.title":   {"Run or preview", "Запуск или preview"},
	"run.preview": {"Preview target command", "Preview команды target"},
	"run.once":    {"Run schedule once", "Запустить расписание один раз"},
	"run.tick":    {"Run foreground tick", "Запустить foreground tick"},
	"run.warning": {"This action invokes an official provider CLI and may consume subscription allowance. Continue?", "Действие вызовет официальный provider CLI и может расходовать subscription allowance. Продолжить?"},

	"history.title":  {"History and status", "История и статус"},
	"history.status": {"Overall status", "Общий статус"},
	"history.all":    {"Execution history", "История запусков"},
	"history.last":   {"Latest schedule run", "Последний запуск расписания"},

	"settings.title":                        {"Settings", "Настройки"},
	"settings.language":                     {"Change language", "Изменить язык"},
	"settings.updates":                      {"Automatic updates", "Автоматические обновления"},
	"settings.notifications":                {"Completion notifications", "Уведомления о завершении"},
	"settings.paths":                        {"Show local data paths", "Показать пути локальных данных"},
	"settings.updates.title":                {"Automatic updates", "Автоматические обновления"},
	"settings.updates.description":          {"Enable one verified update attempt before the selected interval schedule runs? No second timer will be created.", "Включить одну проверенную попытку обновления перед запуском выбранного интервального расписания? Второй таймер создан не будет."},
	"settings.updates.schedule":             {"Choose the interval schedule for updates", "Выберите интервальное расписание для обновлений"},
	"settings.updates.schedule_description": {"Updates use this schedule's existing cadence and phase.", "Обновления используют cadence и phase этого существующего расписания."},
	"settings.updates.no_interval":          {"No interval schedule exists. Automatic updates remain disabled.", "Интервальных расписаний нет. Автоматические обновления остаются выключенными."},
	"settings.notifications.title":          {"Completion notifications", "Уведомления о завершении"},
	"settings.notifications.description":    {"Show a private macOS notification after a scheduled provider attempt? It includes only the schedule, target, and success/failure status.", "Показывать приватное уведомление macOS после завершения запуска провайдера? Оно содержит только расписание, target и статус успеха или ошибки."},
	"settings.notifications.enabled":        {"enabled", "включены"},
	"settings.notifications.disabled":       {"disabled", "выключены"},

	"confirm.remove_schedule":   {"Remove this schedule? Provider credentials will not be changed.", "Удалить это расписание? Credentials провайдера не изменятся."},
	"confirm.uninstall":         {"Uninstall this LaunchAgent?", "Удалить этот LaunchAgent?"},
	"operation.failed":          {"Operation failed: %v", "Ошибка операции: %v"},
	"validation.required":       {"A value is required", "Обязательное значение"},
	"accessible.choose":         {"Choose a number [%d]: ", "Выберите номер [%d]: "},
	"accessible.invalid_choice": {"Enter a number from 1 to %d.", "Введите число от 1 до %d."},
	"accessible.input_default":  {"Value [%s]: ", "Значение [%s]: "},
	"accessible.yes_no":         {"Enter yes or no.", "Введите да или нет."},
}

func normalizeLanguage(value string) string {
	if value == "ru" {
		return "ru"
	}
	return "en"
}

func text(language, key string, args ...any) string {
	value, ok := messages[key]
	if !ok {
		return key
	}
	format := value.EN
	if normalizeLanguage(language) == "ru" {
		format = value.RU
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
