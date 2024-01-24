package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/StackExchange/wmi"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/oschwald/maxminddb-golang"
	"github.com/shirou/gopsutil/disk"
	"golang.org/x/sys/windows/registry"
)

var (
	fileConfig, fileGames, hostname, ipInfo, trialfile string
	serverID, authToken, mmdbASN, mmdbCity, Session_ID string
	isRunning                                          bool
	infoHTML                                           string
)

const (
	newTitle    = "Drova Notifier v2"                                  // Имя окна программы
	UrlSessions = "https://services.drova.io/session-manager/sessions" // инфо по сессиям
	UrlServers  = "https://services.drova.io/server-manager/servers"   // для получения инфо по серверам
)

// для выгрузки названий игр с их ID
type Product struct {
	ProductID string `json:"productId"`
	Title     string `json:"title"`
}

// для получения провайдера в оффлайн базе
type ASNRecord struct {
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// для получения города региона в оффлайн базе
type CityRecord struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Subdivision []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
}

// online инфо по IP
type IPInfoResponse struct {
	IP     string `json:"ip"`
	City   string `json:"city"`
	Region string `json:"region"`
	ISP    string `json:"org"`
}

// структура для выгрузки ID и названия серверов
type serverManager []struct {
	Server_id    string `json:"uuid"`
	Name         string `json:"name"`
	User_id      string `json:"user_id"`
	Status       string `json:"state"`
	Public       bool   `json:"published"`
	SessionStart int64  `json:"alive_since"`
}

// для получения времени запуска windows
type Win32_OperatingSystem struct {
	LastBootUpTime time.Time
}

func main() {
	logFilePath := "log.log" // Имя файла для логирования ошибок
	logFilePath = filepath.Join(filepath.Dir(os.Args[0]), logFilePath)
	// Открываем файл для записи логов
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Println("[ERROR] Ошибка открытия файла", err, getLine())
		restart()
	}
	defer logFile.Close()
	// Получаем текущую директорию программы
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Println("[ERROR] Ошибка получения текущей деректории: ", err, getLine())
		restart()
	}
	// Устанавливаем файл в качестве вывода для логгера
	log.SetOutput(logFile)

	// Получаем имя ПК
	stationName, err := os.Hostname()
	if err != nil {
		log.Println("[ERROR] Ошибка при получении имени компьютера: ", err, getLine())
		return
	}
	hostname = stationName

	time.Sleep(20 * time.Second)
	BotToken, Chat_IDint, UserID, ServiceChatID = getConfigBot(hostname)
	if ServiceChatID == 0 {
		ServiceChatID = Chat_IDint
	}

	log.Println("[INFO] Start program")

	fileGames = filepath.Join(dir, "games.txt")
	fileConfig = filepath.Join(dir, "config.txt")
	mmdbASN = filepath.Join(dir, "GeoLite2-ASN.mmdb")   // файл оффлайн базы IP. Провайдер
	mmdbCity = filepath.Join(dir, "GeoLite2-City.mmdb") // файл оффлайн базы IP. Город и область

	getConfigFile(fileConfig)

	if TrialON {
		if TrialfileLAN != "" {
			_, err = os.Stat(TrialfileLAN)
			if os.IsNotExist(err) {
				// Файл не существует
				log.Printf("[INFO] Файл %s отсутствует или нет доступа к нему\n", TrialfileLAN)
				trialfile = filepath.Join(dir, "trial.txt")
				log.Println("[INFO] Запись триала в ", trialfile)
			} else {
				trialfile = TrialfileLAN
				log.Println("[INFO] Запись триала в ", trialfile)
			}
		} else {
			trialfile = filepath.Join(dir, "trial.txt")
			log.Println("[INFO] Запись триала в ", trialfile)
		}
	} else {
		TrialBlock = false
		TrialfileLAN = ""
	}

	if !OnlineIpInfo {
		_, err = os.Stat(mmdbASN)
		if os.IsNotExist(err) {
			// Файл не существует
			OnlineIpInfo = true
			log.Printf("Файл %s не существует. %s. %s\n", fileConfig, err, getLine())
		} else {
			_, err = os.Stat(mmdbCity)
			if os.IsNotExist(err) {
				// Файл не существует
				OnlineIpInfo = true
				log.Printf("Файл %s не существует. %s. %s\n", fileConfig, err, getLine())
			} else {
				if !AutoUpdateGeolite { // если не включен автоапдейт
					go restartGeoLite(mmdbASN, mmdbCity) // запускаем проверку изменений файлов GeoLite
				} else { // иначе
					updateGeoLite(mmdbASN, mmdbCity) // проверяем есть ли обновление для GeoLite
				}
			}
		}
	}

	gameID(fileGames) // получение списка ID игры - Название игры и сохранение в файл gamesID.txt

	regFolder := `SOFTWARE\ITKey\Esme`
	serverID = regGet(regFolder, "last_server") // получаем ID сервера
	regFolder += `\servers\` + serverID
	authToken = regGet(regFolder, "auth_token") // получаем токен для авторизации
	go validToken(regFolder, authToken)

	antiCheat(hostname, CheckAntiCheat) // проверка античитов
	diskSpace(hostname, CheckFreeSpace) // проверка свободного места на дисках
	messageStartWin(hostname)           // проверка времени запуска станции
	go esmeCheck(hostname)              // запуск мониторинга сервиса дров
	if checkIfProcessRunning("LibreHardwareMonitor.exe") && CheckTempON {
		go CheckHWt(hostname) // мониторинг температур
	} else if !checkIfProcessRunning("LibreHardwareMonitor.exe") && CheckTempON {
		chatMessage := hostname + ". LibreHardwareMonitor не запущен. Приложение перезапустится после запуска LibreHardwareMonitor"
		log.Println("LibreHardwareMonitor не запущен. Приложение перезапустится после запуска LibreHardwareMonitor")
		err := SendMessage(BotToken, Chat_IDint, chatMessage)
		if err != nil {
			log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
		}
		for {
			if checkIfProcessRunning("LibreHardwareMonitor.exe") {
				log.Println("LibreHardwareMonitor запущен. Перезапускаем приложение")
				restart()
			}
			time.Sleep(10 * time.Second)
		}
	}

	if CommandON {
		go commandBot(BotToken, hostname, UserID)
	}

	for {
		for i := 0; i != 2; { //ждем запуска приложения ese.exe
			isRunning = checkIfProcessRunning("ese.exe") // запущено ли приложение
			if isRunning {
				log.Println("[INFO] Старт сессии")
				if StartMessageON {
					chatMessage := sessionInfo("Start")
					err := SendMessage(BotToken, Chat_IDint, chatMessage)
					if err != nil {
						log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
					}
				}
				i = 2 //т.к. приложение запущено, выходим из цикла
			}
			time.Sleep(5 * time.Second) // интервал проверки запущенного процесса
		}
		// ждем закрытия процесса ese.exe
		for i := 0; i != 3; {
			isRunning = checkIfProcessRunning("ese.exe")
			if !isRunning {
				log.Println("[INFO] Завершение сессии")
				if StopMessageON {
					go GetComment("Stop")
				}
				if CommentMessageON {
					go GetComment("Comment")
				}
				antiCheat(hostname, CheckAntiCheat) // проверка античитов
				diskSpace(hostname, CheckFreeSpace) // проверка свободного места на дисках
				if !OnlineIpInfo {
					if AutoUpdateGeolite { // если включен автоапдейт
						updateGeoLite(mmdbASN, mmdbCity) // проверяем обновления файлов GeoLite
					}
				}

				i = 3 // выходим из цикла
			}
			time.Sleep(5 * time.Second) // интервал проверки запущенного процесса
		}
	}
}

// Проверяет, запущен ли указанный процесс
func checkIfProcessRunning(processName string) bool {
	cmd := exec.Command("tasklist")
	output, err := cmd.Output()
	if err != nil {
		log.Println("[ERROR] Ошибка получения списка процессов:", err, getLine())
	}
	return strings.Contains(string(output), processName)
}

// отправка сообщения ботом
func SendMessage(botToken string, chatID int64, text string) error {
	var i int = 0
	var err error
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Println("[ERROR] Ошибка подключения бота: ", err, getLine())
		return err
	}
	bot.Debug = true

	i = 0
	message := tgbotapi.NewMessage(chatID, text)
	message.ParseMode = "HTML"
	for i = 0; i < 3; i++ {
		_, err = bot.Send(message)
		if err != nil {
			log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
			time.Sleep(1 * time.Second)
			return err
		} else if err == nil {
			i = 3
		}
	}

	return nil
}

// получение строки кода где возникла ошибка
func getLine() string {
	_, _, line, _ := runtime.Caller(1)
	lineErr := fmt.Sprintf("\nОшибка в строке: %d", line)
	return lineErr
}

// получение списка игр с их ID
func gameID(fileGames string) {
	// Отправить GET-запрос на API
	respGame, err := http.Get("https://services.drova.io/product-manager/product/listfull2")
	if err != nil {
		fmt.Println("[ERROR] Ошибка при выполнении запроса:", err, getLine())
		return
	}
	defer respGame.Body.Close()

	// Прочитать JSON-ответ
	var products []Product
	err = json.NewDecoder(respGame.Body).Decode(&products)
	if err != nil {
		fmt.Println("[ERROR] Ошибка при разборе JSON-ответа:", err, getLine())
		return
	}
	// Создать файл для записи
	file, err := os.Create(fileGames)
	if err != nil {
		fmt.Println("[ERROR] Ошибка при создании файла:", err, getLine())
		return
	}
	defer file.Close()

	// Записывать данные в файл
	for _, product := range products {
		line := fmt.Sprintf("%s = %s\n", product.ProductID, product.Title)
		_, err = io.WriteString(file, line)
		if err != nil {
			fmt.Println("[ERROR] Ошибка при записи данных в файл:", err, getLine())
			return
		}
	}
	time.Sleep(1 * time.Second)
}

// конвертирование дат
func dateTimeS(data int64) (string, time.Time) {

	// Создание объекта времени
	seconds := int64(data / 1000)
	nanoseconds := int64((data % 1000) * 1000000)
	t := time.Unix(seconds, nanoseconds)

	// Форматирование времени
	formattedTime := t.Format("02-01-2006 15:04:05")

	return formattedTime, t
}

// высчитываем продолжительность сессии
func dur(stopTime, startTime time.Time) (string, int) {
	var minutes int
	var sessionDur string
	if stopTime.String() != "" {
		duration := stopTime.Sub(startTime).Round(time.Second)
		// log.Println("[DIAG]duration - ", duration)
		hours := int(duration.Hours())
		// log.Println("[DIAG]hours - ", hours)
		minutes = int(duration.Minutes()) % 60
		// log.Println("[DIAG]minutes - ", minutes)
		seconds := int(duration.Seconds()) % 60
		// log.Println("[DIAG]seconds - ", seconds)
		hou := strconv.Itoa(hours)
		sessionDur = ""
		if hours < 10 {
			sessionDur = sessionDur + "0" + hou + ":"
		} else {
			sessionDur = sessionDur + hou + ":"
		}
		min := strconv.Itoa(minutes)
		if minutes < 10 {
			sessionDur = sessionDur + "0" + min + ":"
		} else {
			sessionDur = sessionDur + min + ":"
		}
		sec := strconv.Itoa(seconds)
		if seconds < 10 {
			sessionDur = sessionDur + "0" + sec
		} else {
			sessionDur = sessionDur + sec
		}
		if !ShortSessionON {
			if hours == 0 && minutes < minMinute {
				sessionDur = "off"
			}
		}

	} else {
		sessionDur = "[ERROR] Ошибка получения времени окончания сессии"
		log.Println(sessionDur)
	}
	return sessionDur, minutes
}

// получаем данные из реестра
func regGet(regFolder, keys string) string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, regFolder, registry.QUERY_VALUE)
	if err != nil {
		log.Printf("Failed to open registry key: %v. %s\n", err, getLine())
	}
	defer key.Close()

	value, _, err := key.GetStringValue(keys)
	if err != nil {
		log.Printf("Failed to read last_server value: %v. %s\n", err, getLine())
	}

	return value
}

// offline инфо по IP
func getASNRecord(mmdbCity, mmdbASN string, ip net.IP) (*CityRecord, *ASNRecord, error) {
	dbASN, err := maxminddb.Open(mmdbASN)
	if err != nil {
		return nil, nil, err
	}
	defer dbASN.Close()

	var recordASN ASNRecord
	err = dbASN.Lookup(ip, &recordASN)
	if err != nil {
		return nil, nil, err
	}

	db, err := maxminddb.Open(mmdbCity)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	var recordCity CityRecord
	err = db.Lookup(ip, &recordCity)
	if err != nil {
		return nil, nil, err
	}

	var Subdivision CityRecord
	err = db.Lookup(ip, &Subdivision)
	if err != nil {
		return nil, nil, err
	}
	return &recordCity, &recordASN, err
}

// полученные данных из оффлайн базы
func offlineDBip(ip string) string {
	var city, region, asn string = "", "", ""

	cityRecord, asnRecord, err := getASNRecord(mmdbCity, mmdbASN, net.ParseIP(ip))
	if err != nil {
		log.Println(err)
	}

	asn = asnRecord.AutonomousSystemOrganization // провайдер клиента
	if err != nil {
		log.Println(err, getLine())
		asn = ""
	}

	if val, ok := cityRecord.City.Names["ru"]; ok { // город клиента
		city = val
		if err != nil {
			log.Println(err, getLine())
			city = ""
		}
	} else {
		if val, ok := cityRecord.City.Names["en"]; ok {
			city = val
			if err != nil {
				log.Println(err, getLine())
				city = ""
			}
		}
	}

	if len(cityRecord.Subdivision) > 0 {
		if val, ok := cityRecord.Subdivision[0].Names["ru"]; ok { // регион клиента
			region = val
			if err != nil {
				log.Println(err, getLine())
				region = ""
			}
		} else {
			if val, ok := cityRecord.Subdivision[0].Names["en"]; ok {
				region = val
				if err != nil {
					log.Println(err, getLine())
					region = ""
				}
			}
		}
	}

	if city != "" {
		ipInfo = " - " + city
	}
	if region != "" {
		ipInfo += " - " + region
	}
	if asn != "" {
		ipInfo += " - " + asn
	}
	return ipInfo
}

// перезапуск приложения
func restart() {
	// Получаем путь к текущему исполняемому файлу
	execPath, err := os.Executable()
	if err != nil {
		log.Println(err, getLine())
	}

	// Запускаем новый экземпляр приложения с помощью os/exec
	cmd := exec.Command(execPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Запускаем новый процесс и не ждем его завершения
	err = cmd.Start()
	if err != nil {
		log.Println(err, getLine())
	}

	// Завершаем текущий процесс
	os.Exit(0)
}

// оповещение о включении станции
func messageStartWin(hostname string) {
	var osInfo []Win32_OperatingSystem
	err := wmi.Query("SELECT LastBootUpTime FROM Win32_OperatingSystem", &osInfo)
	if err != nil {
		log.Println(err, getLine())
	}

	lastBootUpTime := osInfo[0].LastBootUpTime
	formattedTime := lastBootUpTime.Format("02-01-2006 15:04:05")
	log.Println("[INFO] Windows запущен - ", formattedTime)
	// Получаем текущее время
	currentTime := time.Now()

	// Вычисляем разницу во времени
	duration := currentTime.Sub(lastBootUpTime)

	// Если прошло менее 5 минут с момента запуска Windows
	if duration.Minutes() < 5 {
		var hname string = ""
		if viewHostname {
			hname = hostname + " "
		}
		message := fmt.Sprintf("Внимание! Станция %sзапущена менее 5 минут назад!\nВремя запуска - %s", hname, formattedTime)
		err := SendMessage(BotToken, ServiceChatID, message)
		if err != nil {
			log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
		}
	}
}

// проверяем свободное место на дисках
func diskSpace(hostname string, checkFreeSpace bool) {
	if checkFreeSpace {
		var text string = ""
		partitions, err := disk.Partitions(false)
		if err != nil {
			log.Println(err, getLine())
		}

		for _, partition := range partitions {
			usageStat, err := disk.Usage(partition.Mountpoint)
			if err != nil {
				log.Printf("[ERROR] Ошибка получения данных для диска %s: %v. %s\n", partition.Mountpoint, err, getLine())
				continue
			}

			usedSpacePercent := usageStat.UsedPercent
			freeSpace := float32(usageStat.Free) / (1024 * 1024 * 1024)
			if usedSpacePercent > 90 {
				text += fmt.Sprintf("На диске %s свободно менее 10%%, %.2f Гб\n", partition.Mountpoint, freeSpace)
			}
		}
		var hname string = ""
		if viewHostname {
			hname = fmt.Sprintf(" Станция %s", hostname)
		}
		// Если text не пустой, значит есть диск со свободным местом менее 10%, отправляем сообщение
		if text != "" {
			message := fmt.Sprintf("Внимание!%s\n%s", hname, text)
			log.Print(text)
			err := SendMessage(BotToken, ServiceChatID, message)
			if err != nil {
				log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
			}
		}
	}
}

// проверка файлов античитов
func antiCheat(hostname string, checkAntiCheat bool) {
	var hname string = ""
	if viewHostname {
		hname = fmt.Sprintf(" Станция %s", hostname)
	}
	if checkAntiCheat {
		antiCheat := map[string]string{
			"EasyAntiCheat_EOS": "C:\\Program Files (x86)\\EasyAntiCheat_EOS\\EasyAntiCheat_EOS.exe",
			"EasyAntiCheat":     "C:\\Program Files (x86)\\EasyAntiCheat\\EasyAntiCheat.exe",
		}
		for key, value := range antiCheat {
			filePath := value
			if _, err := os.Stat(filePath); err == nil {
				log.Printf("[INFO] Файл %s в наличии\n", filePath)
			} else if os.IsNotExist(err) {
				log.Printf("[INFO] Внимание!%s\nОтсутствует файл %s", hname, key)
				message := fmt.Sprintf("[INFO] Внимание!%s\nОтсутствует файл %s", hname, key)
				err := SendMessage(BotToken, ServiceChatID, message)
				if err != nil {
					log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
					return
				}
			} else {
				log.Printf("[ERROR] Ошибка проверки файла %s: %s. %s\n", filePath, err, getLine())
			}
		}
	}
}

// trial - создание или обновление записи по ключу(ip)
func createOrUpdateKeyValue(key string, value int) {
	data := readDataFromFile()
	// Проверяем, существует ли уже ключ в файле
	index := -1
	for i, line := range data {
		if strings.HasPrefix(line, key+"=") {
			index = i
			break
		}
	}
	// Если ключ не существует(-1), добавляем новую запись. Иначе, увеличиваем его значение
	newValue := value
	if index != -1 {
		oldValue, _ := strconv.Atoi(strings.Split(data[index], "=")[1])
		newValue = oldValue + value
		data[index] = key + "=" + strconv.Itoa(newValue)
	} else {
		data = append(data, key+"="+strconv.Itoa(newValue))
	}
	writeDataToFile(data)
}

// trial - получаем значение по ключу(ip)
func getValueByKey(key string) int {
	data := readDataFromFile()
	for _, line := range data {
		parts := strings.Split(line, "=")
		if parts[0] == key {
			value, _ := strconv.Atoi(parts[1])
			return value
		}
	}
	return -1 // Возвращаем -1, если ключ не найден
}

// trial - читаем файл построчно и сздаем слайс
func readDataFromFile() []string {
	file, err := os.Open(trialfile)
	if err != nil {
		return []string{}
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines
}

// trial записываем слайс в файл построчно
func writeDataToFile(data []string) {
	file, err := os.OpenFile(trialfile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	for _, line := range data {
		if _, err := file.WriteString(line + "\n"); err != nil {
			log.Println(err)
			return
		}
	}
}

// получаем данные из файла в виде ключ = значение
func readConfig(keys, filename string) (string, error) {
	var gname string
	file, err := os.Open(filename)
	if err != nil {
		log.Println("[ERROR] Ошибка при открытии файла ", filename, ": ", err, getLine())
		return "[ERROR] Ошибка при открытии файла: ", err
	}
	defer file.Close()

	// Создать сканер для чтения содержимого файла построчно
	scanner := bufio.NewScanner(file)

	// Создать словарь для хранения пары "ключ-значение"
	data := make(map[string]string)

	// Перебирать строки из файла
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " = ")
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]
			data[key] = value
		}
	}

	if value, ok := data[keys]; ok {
		gname = value
	}
	return gname, err
}

// перезагрузка ПК
func rebootPC() {
	cmd := exec.Command("shutdown", "/r", "/t", "0")
	err := cmd.Run()
	if err != nil {
		log.Println(err)
		return
	}
}

func commandBot(tokenBot, hostname string, userID int64) {
	var messageT, honame, hname string

	honame = strings.ToLower(hostname)
	bot, err := tgbotapi.NewBotAPI(tokenBot)
	if err != nil {
		log.Println(err)
	}
	if viewHostname {
		hname = hostname + "\n"
	} else {
		hname = ""
	}
	// таймаут обновления бота
	upd := tgbotapi.NewUpdate(0)
	upd.Timeout = 60

	// получаем обновления от API
	updates := bot.GetUpdatesChan(upd)
	if err != nil {
		log.Println(err)
	}

	for update := range updates {
		//проверяем тип обновления - только новые входящие сообщения
		if update.Message != nil {

			if update.Message.From.ID == userID {
				messageT = strings.ToLower(update.Message.Text)

				if strings.Contains(messageT, "/reboot") {
					if strings.Contains(messageT, honame) { // Проверяем, что в тексте упоминается имя ПК
						log.Println("Перезагрузка ПК по команде из телеграмма")
						message := fmt.Sprintf("Станция %s будет перезагружена по команде из телеграмма", hostname)
						err := SendMessage(BotToken, userID, message)
						if err != nil {
							log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
							return
						}
						rebootPC()
					} else {
						anotherPC(hostname)
					}
				} else if strings.Contains(messageT, "/status") {
					var serv serverManager // структура serverManager
					responseData, err := getFromURL(UrlServers, "server_id", serverID)
					log.Println("получили команду /статус")
					if err != nil {
						chatMessage := hostname + " Невозможно получить данные с сайта"
						log.Println("[ERROR] Невозможно получить данные с сайта")
						err := SendMessage(BotToken, userID, chatMessage) // отправка сообщения
						if err != nil {
							log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
						}
					} else {
						json.Unmarshal([]byte(responseData), &serv) // декодируем JSON файл

						var serverName, status, messageText string
						messageText = ""
						i := 0
						// messageText = fmt.Sprint(hname)
						// log.Println("/статус - ошибок нет, собираем данные")
						for range serv {
							// log.Println("/статус - зашли в рэндж серверов")
							var sessionStart, server_ID string
							serverName = serv[i].Name
							status = serv[i].Status // Получаем статус сервера
							server_ID = serv[i].Server_id
							// log.Println(serverName, "-", status, "-", server_ID)

							if status == "BUSY" || status == "HANDSHAKE" { // Получаем время начала, если станция занят
								var data SessionsData // структура SessionsData
								responseData, err := getFromURL(UrlSessions, "server_id", server_ID)
								if err != nil {
									chatMessage := hostname + " Невозможно получить данные с сайта"
									log.Println("[ERROR] Невозможно получить данные с сайта")
									err := SendMessage(BotToken, userID, chatMessage) // отправка сообщения
									if err != nil {
										log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
									}
									sessionStart = ""
								} else {
									json.Unmarshal([]byte(responseData), &data) // декодируем JSON файл
									startTime, _ := dateTimeS(data.Sessions[0].Created_on)
									sessionStart = fmt.Sprintf("\n%s", startTime)
								}
							} else {
								sessionStart = ""
							}
							messageText += fmt.Sprintf("%s - %s%s\n\n", serverName, status, sessionStart)
							i++
						}

						err := SendMessage(BotToken, userID, messageText)
						if err != nil {
							log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
							return
						}
					}
				} else if strings.Contains(messageT, "/visible") {
					if strings.Contains(messageT, honame) { // Проверяем, что в тексте упоминается имя ПК
						err := viewStation("true", serverID)
						if err != nil {
							log.Println("[ERROR] Ошибка смены статуса: ", err)
							message := fmt.Sprintf("Ошибка. Станция %s не видна клиентам. Повторите попытку позже", hostname)
							err = SendMessage(BotToken, userID, message)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						} else {
							log.Printf("Станция %s в сети\n", hostname)
							message := fmt.Sprintf("Станция %s видна клиентам", hostname)
							err = SendMessage(BotToken, userID, message)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						}
					} else {
						anotherPC(hostname)
					}
				} else if strings.Contains(messageT, "/invisible") {
					if strings.Contains(messageT, honame) { // Проверяем, что в тексте упоминается имя ПК
						err := viewStation("false", serverID)
						if err != nil {
							log.Println("[ERROR] Ошибка смены статуса: ", err)
							message := fmt.Sprintf("Ошибка. Станция %s не спрятана от клиентов. Повторите попытку позже", hostname)
							err = SendMessage(BotToken, userID, message)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						} else {
							log.Printf("Станция %s спрятана\n", hostname)
							message := fmt.Sprintf("Станция %s спрятана от клиентов", hostname)
							err = SendMessage(BotToken, userID, message)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						}
					} else {
						anotherPC(hostname)
					}
				} else if strings.Contains(messageT, "/temp") {
					log.Println("Получение температур и оборотов вентиляторов")
					var message string
					_, _, _, _, _, _, _, message = GetTemperature()

					message = hname + message
					err := SendMessage(BotToken, userID, message)
					if err != nil {
						log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
						return
					}
				} else if strings.Contains(messageT, "/delayreboot") {
					if strings.Contains(messageT, honame) { // Проверяем, что в тексте упоминается имя ПК
						go delayReboot(0)
						message := fmt.Sprintf("Будет выполнена перезагрузка %sпо окончании сессии", hname)
						err := SendMessage(BotToken, userID, message)
						if err != nil {
							log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
							return
						}
					} else {
						anotherPC(hostname)
					}
				} else if strings.Contains(messageT, "/drovastop") {
					if strings.Contains(messageT, honame) { // Проверяем, что в тексте упоминается имя ПК
						err := drovaService("stop")
						if err != nil {
							message := fmt.Sprintf("%sОшибка завершения задачи Streaming Service", hname)
							err := SendMessage(BotToken, userID, message)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						} else {
							message := fmt.Sprintf("%sЗадача Streaming Service остановлена", hname)
							err := SendMessage(BotToken, userID, message)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						}
					} else {
						anotherPC(hostname)
					}
				} else if strings.Contains(messageT, "/drovastart") {
					if strings.Contains(messageT, honame) { // Проверяем, что в тексте упоминается имя ПК
						err := drovaService("start")
						if err != nil {
							message := fmt.Sprintf("%sОшибка запуска задачи Streaming Service", hname)
							err := SendMessage(BotToken, userID, message)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						} else {
							message := fmt.Sprintf("%sЗадача Streaming Service запущена", hname)
							err := SendMessage(BotToken, userID, message)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						}
					} else {
						anotherPC(hostname)
					}
				} else if strings.Contains(messageT, "/start") {
					message := fmt.Sprintln("Доступные комманды. ST1 имя вашего ПК")
					message += fmt.Sprintln("/rebootST1 - перезагрузить ST1")
					message += fmt.Sprintln("/delayrebootST1 - перезагрузка ST1 когда закончится сессия")
					message += fmt.Sprintln("/visibleST1 - скрыть ST1")
					message += fmt.Sprintln("/invisibleST1 - скрыть ST1")
					message += fmt.Sprintln("/status - статус серверов")
					message += fmt.Sprintln("/temp - температуры")
					// message += fmt.Sprintln("/drovastartST1 - старт Streaming Service ST1")
					// message += fmt.Sprintln("/drovastopST1 - стоп Streaming Service ST1")
					// message += fmt.Sprintln("")
					// message += fmt.Sprintln("")

					err := SendMessage(BotToken, userID, message)
					if err != nil {
						log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
						return
					}
				} else {
					messageText := "Неизвестная команда"
					err := SendMessage(BotToken, userID, messageText)
					if err != nil {
						log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
						return
					}
				}
			}
			log.Printf("Сообщение от %d: %s", update.Message.From.ID, update.Message.Text)
		}
	}
}

func getFromURL(url, cell, IDinCell string) (responseString string, err error) {
	_, err = http.Get("https://services.drova.io")
	if err != nil {
		log.Println("[ERROR] Сайт https://services.drova.io недоступен")
		return
	} else {
		// Создание HTTP клиента
		client := &http.Client{}

		var resp *http.Response

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Println("[ERROR] Ошибка создания запроса: ", err, getLine())
			return "", err
		}

		// Установка параметров запроса
		q := req.URL.Query()
		q.Add(cell, IDinCell)
		req.URL.RawQuery = q.Encode()

		// Установка заголовка X-Auth-Token
		req.Header.Set("X-Auth-Token", authToken)

		// Отправка запроса и получение ответа
		resp, err = client.Do(req)
		if err != nil {
			log.Println("[ERROR] Ошибка отправки запроса: ", err, getLine())
			return "", err
		}
		defer resp.Body.Close()
		// Запись ответа в строку
		var buf bytes.Buffer
		_, err = io.Copy(&buf, resp.Body)
		if err != nil {
			log.Println("[ERROR] Ошибка записи запроса в буффер: ", err, getLine())
			return "", err
		}

		responseString = buf.String()
	}

	return responseString, err
}

// получаем IP интерфейса с наибольшей скоростью исходящего трафика
func getInterface() (localAddr, nameInterface string) {

	var localIP, maxInterfaceName string
	var maxOutgoingSpeed float64
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("[ERROR] Ошибка получения интерфейсов. %s. %s\n", err, getLine())
	}

	maxInterfaceName, maxOutgoingSpeed = getSpeed()

	for _, interf := range interfaces {
		addrs, err := interf.Addrs()
		if err != nil {
			log.Printf("[ERROR] Ошибка получения ip адресов. %s. %s\n", err, getLine())
		}
		for _, add := range addrs {
			if ip, ok := add.(*net.IPNet); ok {
				localIP = ip.String()
			}
		}

		if interf.Name == maxInterfaceName {
			localAddr = localIP
		}
	}
	log.Printf("[INFO] Интерфейс с макс. исх. скоростью: %s, IP: %s, скорость: %.0f байт/сек\n", maxInterfaceName, localAddr, maxOutgoingSpeed)
	return localAddr, maxInterfaceName
}

// Проверяем запущен ли Drova service
func esmeCheck(hostname string) {
	var i, y uint8 = 0, 0
	for {
		// если процесс не запущен, с каждой следующей проверкой увеличиваем задержку отправки сообщения
		// используя переменную i. 2-е оповещение через 20минут после первого, 3-е через 30минут после второго
		// после отправки 3х сообщений, отправляем оповещение\напоминание с интервалом в 2часа
		if i < 3 {
			for y = 0; y <= i; y++ {
				time.Sleep(5 * time.Minute) // интервал проверки
			}
		} else {
			time.Sleep(60 * time.Minute) // интервал проверки
		}

		statusSession, statusServer, public, err := statusServSession()
		if err != nil {
			log.Println("[ERROR] Ошибка получения статусов: ", err, getLine())
		} else {
			if !checkIfProcessRunning("esme.exe") || (statusServer == "OFFLINE" && public) { // если сервис не запущен
				var chatMessage string
				time.Sleep(2 * time.Minute)
				_, statusServer, _, err := statusServSession()
				if err != nil {
					log.Println("[ERROR] Ошибка получения статусов: ", err, getLine())
				} else {
					if statusServer == "OFFLINE" {
						chatMessage = fmt.Sprintf("ВНИМАНИЕ! Станции %s offline\n", hostname) // формируем сообщение
						chatMessage += fmt.Sprintf("Статус сессии - %s\n", statusSession)
						err := SendMessage(BotToken, ServiceChatID, chatMessage) // отправка сообщения
						if err != nil {
							log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
						}
						go delayReboot(10)
						log.Printf("[INFO] Станции %s offline\n", hostname) // записываем в лог
						i++                                                 // ведем счет отправленных сообщений
					}
				}
			} else {
				i, y = 0, 0
			}
		}
	}
}

// проверка на валидность токена
func validToken(regFolder, authToken string) {
	for {
		authTokenV := regGet(regFolder, "auth_token") // получаем токен для авторизации
		if authToken != authTokenV {
			log.Println("[INFO] Токен не совпадает, перезапуск приложения")
			restart()
		}
		time.Sleep(5 * time.Minute)
	}
}

func anotherPC(hostname string) {
	messageText := fmt.Sprintf("Имя ПК не совпадает: %s\n", hostname)
	err := SendMessage(BotToken, Chat_IDint, messageText)
	if err != nil {
		log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
	}
}

// скрыть\отобразить станцию
func viewStation(seeSt, serverID string) error {
	resp, err := http.Get("https://services.drova.io")
	if err != nil {
		fmt.Println("Сайт недоступен")
	} else {
		if resp.StatusCode == http.StatusOK {
			url := "https://services.drova.io/server-manager/servers/" + serverID + "/set_published/" + seeSt

			request, err := http.NewRequest("POST", url, nil)
			if err != nil {
				fmt.Println("Ошибка при создании запроса:", err)
				return err
			}

			request.Header.Set("X-Auth-Token", authToken) // Установка заголовка X-Auth-Token

			client := &http.Client{}
			response, err := client.Do(request)
			if err != nil {
				fmt.Println("Ошибка при отправке запроса:", err)
				return err
			}
			defer response.Body.Close()
		}
	}
	return err
}

func GetComment(status string) {
	chatMessage := sessionInfo(status) // формируем сообщение с комментарием
	if status == "Comment" {
		err := SendMessage(BotToken, ServiceChatID, chatMessage) // отправка сообщения
		if err != nil {
			log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
		}
	} else if chatMessage != "off" && chatMessage != "" {
		err := SendMessage(BotToken, Chat_IDint, chatMessage) // отправка сообщения
		if err != nil {
			log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
		}
	}
}

// func restartService() {
// 	command := "\\Drova\\Streaming Service"
// 	cmd := exec.Command("schtasks", "/end", "/tn", command)
// 	err := cmd.Run()
// 	if err != nil {
// 		fmt.Println("[ERROR] Ошибка выполнения команды:", err)
// 		return
// 	}
// 	fmt.Println("Команда успешно выполнена")
// 	time.Sleep(2 * time.Second)
// 	cmd = exec.Command("schtasks", "/run", "/tn", command)
// 	err = cmd.Run()
// 	if err != nil {
// 		fmt.Println("[ERROR] Ошибка выполнения команды:", err)
// 		return
// 	}
// 	fmt.Println("Команда успешно выполнена")
// }

func statusServSession() (statusSession, statusServer string, public bool, err error) {
	responseStringServers, err := getFromURL(UrlServers, "uuid", serverID)
	if err != nil {
		chatMessage := hostname + " Невозможно получить данные с сайта"
		log.Println("[ERROR] Невозможно получить данные с сайта")
		err := SendMessage(BotToken, ServiceChatID, chatMessage) // отправка сообщения
		if err != nil {
			log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
		}
	} else {
		var serv serverManager                               // структура serverManager
		json.Unmarshal([]byte(responseStringServers), &serv) // декодируем JSON файл

		var x, y int8 = 0, 0

		for range serv {
			if serv[x].Server_id == serverID {
				y = x
			}
			x++
		}

		responseStringSessions, err := getFromURL(UrlSessions, "server_id", serverID)
		if err != nil {
			chatMessage := hostname + "невозможно получить данные с сайта"
			log.Println("[ERROR] Невозможно получить данные с сайта")
			err := SendMessage(BotToken, ServiceChatID, chatMessage) // отправка сообщения
			if err != nil {
				log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
			}
		} else {
			var data SessionsData                                 // структура SessionsData
			json.Unmarshal([]byte(responseStringSessions), &data) // декодируем JSON файл
			statusSession = data.Sessions[0].Status
			statusServer = serv[y].Status
			public = serv[y].Public
		}
	}
	return statusSession, statusServer, public, err
}

func delayReboot(n int) {
	for {
		statusSession, statusServer, _, err := statusServSession()
		if err != nil {
			log.Println("[ERROR] Ошибка получения статусов: ", err, getLine())
		} else {
			var i int
			if statusSession != "ACTIVE" {
				chatMessage := fmt.Sprintf("Станция %s %s\n", hostname, statusServer)
				chatMessage += fmt.Sprintf("Статус сессии - %s", statusSession)
				err := SendMessage(BotToken, ServiceChatID, chatMessage) // отправка сообщения
				if err != nil {
					log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
				}
				for i = 0; i <= n; i++ {
					_, statusServer, _, err := statusServSession()
					if err != nil {
						log.Println("[ERROR] Ошибка получения статусов: ", err, getLine())
					} else {
						if (statusServer == "OFFLINE" && i == n) || n == 0 {
							chatMessage := fmt.Sprintf("Станция %s будет перезагружена через минуту", hostname)
							err := SendMessage(BotToken, ServiceChatID, chatMessage) // отправка сообщения
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
							}
							time.Sleep(1 * time.Minute)
							log.Println("[INFO] Станция offline, сессия завершена. Перезагружаем сервер")
							rebootPC()
						} else if statusServer != "OFFLINE" {
							i = n + 1
						}
					}
					time.Sleep(1 * time.Minute)
				}
				if i > n {
					break
				}
			}
		}
		time.Sleep(1 * time.Minute)
	}
}

func drovaService(command string) (err error) {
	path := "\\Drova\\Streaming Service"
	if command == "stop" {
		cmd := exec.Command("schtasks", "/end", "/tn", path)
		err = cmd.Run()
		if err != nil {
			fmt.Println("[ERROR] Ошибка выполнения команды:", err)
			return
		}
		log.Println("Команда успешно выполнена")
	}

	if command == "start" {
		cmd := exec.Command("schtasks", "/run", "/tn", path)
		err = cmd.Run()
		if err != nil {
			fmt.Println("[ERROR] Ошибка выполнения команды:", err)
			return
		}
		log.Println("Команда успешно выполнена")
	}
	return
}
