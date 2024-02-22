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

	"github.com/oschwald/maxminddb-golang"
)

var (
	fileConfig, fileGames, hostname, ipInfo, trialfile string
	serverID, authToken, mmdbASN, mmdbCity, Session_ID string
	isRunning                                          bool
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
		_, err := SendMessage(BotToken, ServiceChatID, chatMessage, 0)
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
	var messageID int
	var chatMessage string
	for {
		for i := 0; i != 2; { //ждем запуска приложения ese.exe
			isRunning = checkIfProcessRunning("ese.exe") // запущено ли приложение
			if isRunning {
				log.Println("[INFO] Старт сессии")
				chatMessage = sessionInfo("Start")
				if StartMessageON {
					messageID, err = SendMessage(BotToken, Chat_IDint, "<b>🟥</b>"+chatMessage, 0)
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
					go GetComment("Stop", messageID, chatMessage)
				}
				if CommentMessageON {
					go GetComment("Comment", messageID, "")
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

	var localIP, maxInterfaceName, linkSpeed string
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
			linkSpeed = getLinkSpeed(interf.Name)
		}
	}
	log.Printf("[INFO] Интерфейс с макс. исх. скоростью: %s, IP: %s, скорость: %.0f байт/сек\n", maxInterfaceName, localAddr, maxOutgoingSpeed)
	return localAddr, maxInterfaceName + linkSpeed
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

func GetComment(status string, messageID int, infoSession string) {
	chatMessage := sessionInfo(status) // формируем сообщение с комментарием
	if status == "Comment" {
		if chatMessage != "" {
			_, err := SendMessage(BotToken, ServiceChatID, "<b>✍️</b>"+chatMessage, 0) // отправка сообщения
			if err != nil {
				log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
			}
		}
	} else if chatMessage != "off" && chatMessage != "" {
		_, err := SendMessage(BotToken, Chat_IDint, "<b>🟩</b>"+infoSession+chatMessage, messageID) // отправка сообщения
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
		_, err := SendMessage(BotToken, ServiceChatID, chatMessage, 0) // отправка сообщения
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
			_, err := SendMessage(BotToken, ServiceChatID, chatMessage, 0) // отправка сообщения
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
				_, err := SendMessage(BotToken, ServiceChatID, chatMessage, 0) // отправка сообщения
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
							_, err := SendMessage(BotToken, ServiceChatID, chatMessage, 0) // отправка сообщения
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
