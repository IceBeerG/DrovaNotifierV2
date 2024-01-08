package main

import (
	"log"
	"os"
	"strconv"
)

var ( // true - включение функции, false - выключение
	BotToken      string // токен бота
	Chat_IDint    int64  // определяем ID чата получателя
	UserID        int64  // ID пользователя, от которого принимаются команды
	ServiceChatID int64  // чат для сервисных сообщений

	CommandON    bool = true // включить команды управления ботом
	viewHostname bool = true // указывать имя станции в сообщениях
	oneBot4all   bool = true // для всех станций используем 1 учетку бота

	OnlineIpInfo      bool = true  // инфо по IP online
	AutoUpdateGeolite bool = false // автообновление файлов GeoLite с Github

	CheckAntiCheat bool    = true // проверка наличия файлов EasyAntiCheat.exe и EasyAntiCheat_EOS.exe
	CheckFreeSpace bool    = true // проверка свободного места на дисках
	CheckTempON    bool    = true // мониторинг температур
	FANt           float64 = 75   // порог проверки работы вентиляторов видеокарты
	FANrpm         float64 = 800  // минимальные обороты при FANt
	CPUtmax        float64 = 85   // порог температуры процессора
	GPUtmax        float64 = 85   // порог температуры ядра видеокарты
	GPUhsTmax      float64 = 90   // порог температуры HotSpot видеокарты
	DeltaT         float64 = 5    // дельта среднего значения температур от от порога предупреждения. Для сообщения о нормализации температур

	TrialON      bool   = false // сбор статистики по триальщикам в trial.txt. false - не собирается статистика в trial.txt
	TrialBlock   bool   = false // Блокировка "хитрых" триальщиков. false - нет блокировки
	TrialfileLAN string = ``    // файл в сети пример `S:\trial.txt`

	StartMessageON   bool = true // включение сообщений при начале сессии. false - сообщение не будет приходить
	StopMessageON    bool = true // включение о сообщении об окончании сессии. false - сообщение не будет приходить
	ShortSessionON   bool = true // оповещать о сессиях менее Х минут, выставлять ниже. false - сообщение не будет приходить
	minMinute        int  = 10   // выставляем порог отправки сообщений о сессии. значения от 0 до 59
	CommentMessageON bool = true // сообщение с комментарием клиента. false - сообщение не будет приходить
)

func getConfigBot(hostname string) (BotToken string, Chat_IDint, UserID, serviceChatID int64) {

	Chat_IDint = -56743243234 // чат, куда будут приходить информация
	UserID = 3453523234234    // пользователь, от которого будут приниматься команды
	ServiceChatID = 0         // чат для сервисных сообщений, 0 - отправка в Chat_IDint

	if oneBot4all {
		BotToken = "123123:qweq" // токен для станции
	} else {
		station1 := "Station1"      // имя ПК на станции1
		botToken1 := "11111:qweq"   // токен для станции1
		station2 := "Station2"      // имя ПК на станции2
		botToken2 := "22222:qweq"   // токен для станции2
		station3 := "Station3"      // имя ПК на станции3
		botToken3 := "33333:qweq"   // токен для станции3
		station4 := "Station4"      // имя ПК на станции4
		botToken4 := "1325523:qweq" // токен для станции4
		station5 := "Station5"      // имя ПК на станции5
		botToken5 := "1222253:qweq" // токен для станции5
		station6 := "Station6"      // имя ПК на станции6
		botToken6 := "1244443:qweq" // токен для станции6
		station7 := "Station7"      // имя ПК на станции7
		botToken7 := "123123:qweq"  // токен для станции7
		station8 := "Station8"      // имя ПК на станции8
		botToken8 := "123123:qweq"  // токен для станции8
		station9 := "Station9"      // имя ПК на станции9
		botToken9 := "123123:qweq"  // токен для станции9
		station10 := "Station10"    // имя ПК на станции10
		botToken10 := "123123:qweq" // токен для станции10

		switch hostname {
		case station1:
			BotToken = botToken1
		case station2:
			BotToken = botToken2
		case station3:
			BotToken = botToken3
		case station4:
			BotToken = botToken4
		case station5:
			BotToken = botToken5
		case station6:
			BotToken = botToken6
		case station7:
			BotToken = botToken7
		case station8:
			BotToken = botToken8
		case station9:
			BotToken = botToken9
		case station10:
			BotToken = botToken10
		default:
			BotToken = botToken1
		}
	}

	return BotToken, Chat_IDint, UserID, ServiceChatID
}

func getConfigFile(fileConfig string) {
	//блок для получения данных из конфига
	_, err := os.Stat(fileConfig)
	if os.IsNotExist(err) {
		// Файл не существует
		log.Printf("[INFO] Файл %s отсутствует\n", fileConfig)
	} else {

		bToken, err := readConfig("Tokenbot", fileConfig) // определяем токен бота
		if err != nil {
			log.Printf("[ERROR] Ошибка - %s. %s\n", err, getLine())
		}
		if bToken != "" {
			BotToken = bToken // получаем токен этого бота
		}

		ChatIDint := takeConfInt("ChatID", fileConfig)
		if ChatIDint != 0 {
			Chat_IDint = ChatIDint
		}

		SChatID := takeConfInt("ServiceChatID", fileConfig)
		if SChatID != 0 {
			ServiceChatID = SChatID
		}

		UID := takeConfInt("UserID", fileConfig)
		if UID != 0 {
			UserID = UID
		}

		var chk bool

		_, chk = takeConfBool("OnlineIpInfo") // настройки получения инфо по IP
		if chk {
			OnlineIpInfo, _ = takeConfBool("OnlineIpInfo")
		}
		log.Println("OnlineIpInfo - ", OnlineIpInfo)

		_, chk = takeConfBool("CheckFreeSpace") // проверка свободного места на дисках
		if chk {
			OnlineIpInfo, _ = takeConfBool("CheckFreeSpace")
		}
		log.Println("CheckFreeSpace - ", CheckFreeSpace)

		_, chk = takeConfBool("CheckAntiCheat") // проверка папок античитов
		if chk {
			CheckAntiCheat, _ = takeConfBool("CheckAntiCheat")
		}
		log.Println("CheckAntiCheat - ", CheckAntiCheat)

		_, chk = takeConfBool("CommandON") // управление ботом через чат ТГ
		if chk {
			CommandON, _ = takeConfBool("CommandON")
		}
		log.Println("CommandON - ", CommandON)

		_, chk = takeConfBool("StartMessageON") // сообщения о начале сессии
		if chk {
			StartMessageON, _ = takeConfBool("StartMessageON")
		}
		log.Println("StartMessageON - ", StartMessageON)

		_, chk = takeConfBool("StopMessageON") // сообщения об окончании сессии
		if chk {
			StopMessageON, _ = takeConfBool("StopMessageON")
		}
		log.Println("StopMessageON - ", StopMessageON)

		_, chk = takeConfBool("ShortSessionON") // короткие сообщения
		if chk {
			ShortSessionON, _ = takeConfBool("ShortSessionON")
		}
		log.Println("ShortSessionON - ", ShortSessionON)

		_, chk = takeConfBool("CommentMessageON") // включить комментарии
		if chk {
			CommentMessageON, _ = takeConfBool("CommentMessageON")
		}
		log.Println("CommentMessageON - ", CommentMessageON)

		_, chk = takeConfBool("CheckTempON") // включить комментарии
		if chk {
			CheckTempON, _ = takeConfBool("CheckTempON")
		}
		log.Println("CheckTempON - ", CheckTempON)

		if CheckTempON {
			var chkF float64 = 0
			chkF = takeConfFloat("FANt", fileConfig) // порог проверки работы вентиляторов видеокарты
			if chkF != 0 {
				FANt = chkF
			}
			log.Println("FANt - ", FANt)

			chkF = takeConfFloat("FANrpm", fileConfig) // минимальные обороты при FANt
			if chkF != 0 {
				FANrpm = chkF
			}
			log.Println("FANrpm - ", FANrpm)

			chkF = takeConfFloat("CPUtmax", fileConfig) // порог температуры процессора
			if chkF != 0 {
				CPUtmax = chkF
			}
			log.Println("CPUtmax - ", CPUtmax)

			chkF = takeConfFloat("GPUtmax", fileConfig) // порог температуры ядра видеокарты
			if chkF != 0 {
				GPUtmax = chkF
			}
			log.Println("GPUtmax - ", GPUtmax)

			chkF = takeConfFloat("GPUhsTmax", fileConfig) // порог температуры HotSpot видеокарты
			if chkF != 0 {
				GPUhsTmax = chkF
			}
			log.Println("GPUhsTmax - ", GPUhsTmax)
		}

		_, chk = takeConfBool("TrialON") // включить комментарии
		if chk {
			TrialON, _ = takeConfBool("TrialON")
		}
		log.Println("TrialON - ", TrialON) // вести статистику триала

		if TrialON {
			_, chk = takeConfBool("TrialBlock") // включить комментарии
			if chk {
				TrialBlock, _ = takeConfBool("TrialBlock")
			}
			log.Println("TrialBlock -", TrialBlock) // блокировка триальщиков

			TrialfileLAN, err = readConfig("TrialfileLAN", fileConfig) // определяем токен бота
			if err != nil {
				log.Printf("[ERROR] Ошибка - %s. %s\n", err, getLine())
			}
		}
	}
}

func takeConfInt(param, file string) (paramInt int64) {
	paramValue, err := readConfig(param, file) // определяем ID чата
	if err != nil {
		log.Printf("[ERROR] ServiceChatID - %s. %s\n", err, getLine())
	}
	if paramValue != "" {
		paramInt, err = strconv.ParseInt(paramValue, 10, 64) // конвертируем ID чата в int64
		if err != nil {
			log.Printf("[ERROR] %s:  %s. %s\n", paramValue, err, getLine())
		}
	}
	return paramInt
}

// получение данны из файла конфига
func takeConfBool(key string) (value, chk bool) {
	check, err := readConfig(key, fileConfig)
	if err != nil {
		log.Printf("[ERROR] Ошибка - %s. %s\n", err, getLine())
	}
	if check == "true" {
		value = true
		chk = true
	} else if check == "false" {
		value = false
		chk = true
	} else {
		chk = false
	}
	return
}

func takeConfFloat(param, file string) (paramFloat float64) {
	paramValue, err := readConfig(param, file) // определяем ID чата
	if err != nil {
		log.Printf("[ERROR] ServiceChatID - %s. %s\n", err, getLine())
	}
	if paramValue != "" {
		paramFloat, err = strconv.ParseFloat(paramValue, 64) // конвертируем ID чата в int64
		if err != nil {
			log.Printf("[ERROR] %s:  %s. %s\n", paramValue, err, getLine())
		}
	}
	return paramFloat
}
