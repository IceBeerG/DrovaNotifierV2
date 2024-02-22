package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// структура для выгрузки информации по сессиям
type SessionsData struct {
	Sessions []struct {
		Session_uuid  string `json:"uuid"`
		Client_id     string `json:"client_id"`
		Product_id    string `json:"product_id"`
		Created_on    int64  `json:"created_on"`
		Finished_on   int64  `json:"finished_on"` //or null
		Status        string `json:"status"`
		Creator_ip    string `json:"creator_ip"`
		Abort_comment string `json:"abort_comment"` //or null
		Score         string `json:"score"`         //or null
		ScoreReason   string `json:"score_reason"`  //or null
		Comment       string `json:"score_text"`    //or null
		Billing_type  string `json:"billing_type"`  // or null
	}
}

func sessionInfo(status string) (infoString string) {
	var sumTrial int
	var serverIP string
	var hname string = ""
	if viewHostname {
		hname = hostname + " - "
	}
	if status == "Start" { // формируем текст для отправки
		responseString, err := getFromURL(UrlSessions, "server_id", serverID)
		if err != nil {
			infoString = hname + "невозможно получить данные с сайта"
			log.Println("[ERROR] Невозможно получить данные с сайта")
		} else {
			var data SessionsData                         // структура SessionsData
			json.Unmarshal([]byte(responseString), &data) // декодируем JSON файл
			Session_ID = data.Sessions[0].Session_uuid
			log.Printf("[INFO] Подключение %s, billing: %s\n", data.Sessions[0].Creator_ip, data.Sessions[0].Billing_type)
			game, _ := readConfig(data.Sessions[0].Product_id, fileGames)
			sessionOn, _ := dateTimeS(data.Sessions[0].Created_on)
			ipInfo = ""

			if OnlineIpInfo {
				ipInfo = data.Sessions[0].Creator_ip + onlineDBip(data.Sessions[0].Creator_ip)
			} else {
				ipInfo = data.Sessions[0].Creator_ip + offlineDBip(data.Sessions[0].Creator_ip)
			}
			var billing string
			billing = data.Sessions[0].Billing_type
			if billing != "" && billing != "trial" {
				billing = data.Sessions[0].Billing_type
			}

			if TrialON {
				if billing == "trial" {
					sumTrial = getValueByKey(data.Sessions[0].Creator_ip)
					if sumTrial == -1 { // нет записей по этому IP
						createOrUpdateKeyValue(data.Sessions[0].Creator_ip, 0)
						billing = data.Sessions[0].Billing_type
					} else if sumTrial >= 0 && sumTrial < 19 { // уже подключался, но не играл в общей сложности 19 минуту
						billing = fmt.Sprintf("TRIAL %dмин", sumTrial)
					} else if sumTrial > 18 { // начал злоупотреблять
						billing = fmt.Sprintf("TRIAL %dмин\nЗлоупотребление Триалом!", sumTrial)

						if TrialBlock {
							text := "Злоупотребление Триалом! Кикаем!"
							var message string
							if viewHostname {
								message = fmt.Sprintf("Внимание! Станция %s.\n%s", hostname, text)
							} else {
								message = fmt.Sprintf("Внимание!\n%s", text)
							}
							_, err := SendMessage(BotToken, Chat_IDint, message, 0)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
							}
							log.Printf("[INFO] Заблокировано соединение: %s. Trial %d", data.Sessions[0].Creator_ip, sumTrial)
							time.Sleep(10 * time.Second)
							err = runCommand("taskkill", "/IM", "ese.exe", "/F") // закрываем стример сервиса
							if err != nil {
								log.Println("[ERORR] Ошибка выполнения команды:", err)
								return
							}
						}
					}
				}
			}
			localAddr, nameInterface := getInterface()
			serverIP = "\n" + nameInterface + " - " + localAddr
			game = fmt.Sprintf("<b><i> %s </i></b>", game)
			infoString = hname + game + "\n" + ipInfo + "\n" + sessionOn + " - " + billing + serverIP
		}
	} else if status == "Stop" { // высчитываем продолжительность сессии и формируем текст для отправки
		var minute int
		var sessionDur string
		var stopTime, startTime time.Time

		session_ID := Session_ID

		for i := 0; i < 12; i++ {
			responseString, err := getFromURL(UrlSessions, "uuid", session_ID)
			if err != nil {
				log.Println("[ERROR] Stop. Невозможно получить данные с сайта")
			} else {
				var data SessionsData
				json.Unmarshal([]byte(responseString), &data) // декодируем JSON файл
				test := data.Sessions[0].Finished_on
				if test == 0 {
					time.Sleep(5 * time.Second)
					text := fmt.Sprintf("stopTime = %d", data.Sessions[0].Finished_on)
					sessionDur = text
				} else {
					_, stopTime = dateTimeS(data.Sessions[0].Finished_on)
					_, startTime = dateTimeS(data.Sessions[0].Created_on)
					sessionDur, minute = dur(stopTime, startTime)
					i = 12
				}
			}
		}

		responseString, err := getFromURL(UrlSessions, "uuid", Session_ID)
		if err != nil {
			infoString = hname + "невозможно получить данные с сайта"
		} else {
			var dataS SessionsData                         // структура SessionsData
			json.Unmarshal([]byte(responseString), &dataS) // декодируем JSON файл
			log.Printf("[INFO] Отключение %s\n", dataS.Sessions[0].Creator_ip)
			billing := dataS.Sessions[0].Billing_type
			if sessionDur != "off" {
				var billingTrial string = ""
				if TrialON {
					if billing == "trial" {
						sumTrial = getValueByKey(dataS.Sessions[0].Creator_ip)
						if sumTrial < 20 || !TrialBlock {
							ipTrial := dataS.Sessions[0].Creator_ip
							handshake := dataS.Sessions[0].Abort_comment
							if !strings.Contains(handshake, "handshake") { // если кнопка "Играть тут" активированна, добавляем время в файл
								createOrUpdateKeyValue(ipTrial, minute)
							}
							sumTrial = getValueByKey(dataS.Sessions[0].Creator_ip)
							billingTrial = fmt.Sprintf("\nTrial %dмин", sumTrial)
						} else if sumTrial > 20 && TrialBlock {
							billingTrial = fmt.Sprintf("\nKICK - Trial %dмин", sumTrial)
						}
					}
				}
				var comment string
				if dataS.Sessions[0].Abort_comment != "" {
					comment = " - " + dataS.Sessions[0].Abort_comment
				}
				infoString = "\n" + sessionDur + comment + billingTrial
			} else {
				infoString = "off"
			}
		}
	} else if status == "Comment" { // проверяем написание коммента
		var sessionDur, commentC, game string
		var stopTime, startTime time.Time
		var dataC SessionsData

		session_ID := Session_ID

		for i := 0; i < 18; i++ {
			responseString, err := getFromURL(UrlSessions, "uuid", session_ID)
			if err != nil {
				infoString = hname + "невозможно получить данные с сайта"
				log.Println("[ERROR] Невозможно получить данные с сайта")
			} else {
				json.Unmarshal([]byte(responseString), &dataC) // декодируем JSON файл
				if dataC.Sessions[0].Comment == "" {
					time.Sleep(10 * time.Second)
				} else {
					log.Printf("[INFO] Отключение %s\n", dataC.Sessions[0].Creator_ip)
					game, _ = readConfig(dataC.Sessions[0].Product_id, fileGames)
					_, stopTime = dateTimeS(dataC.Sessions[0].Finished_on)
					_, startTime = dateTimeS(dataC.Sessions[0].Created_on)
					sessionDur, _ = dur(stopTime, startTime)
					commentC = dataC.Sessions[0].Comment
					log.Printf("[INFO] Получение комментария %s\n, %s ", dataC.Sessions[0].Creator_ip, session_ID)
					infoString = hname + "<b><i>" + game + "</i></b>" + "\n" + dataC.Sessions[0].Creator_ip + " - " + sessionDur + "\n" + commentC
					i = 18
				}
			}
		}
	}
	return infoString
}

func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
