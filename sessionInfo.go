package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// структура для выгрузки информации по сессиям
type SessionsData struct {
	Sessions []struct {
		Session_uuid  string `json:"uuid"`
		Product_id    string `json:"product_id"`
		Created_on    int64  `json:"created_on"`
		Finished_on   int64  `json:"finished_on"` //or null
		Status        string `json:"status"`
		Creator_ip    string `json:"creator_ip"`
		Abort_comment string `json:"abort_comment"` //or null
		Score         int64  `json:"score"`         //or null
		ScoreReason   int64  `json:"score_reason"`  //or null
		Comment       string `json:"score_text"`    //or null
		Billing_type  string `json:"billing_type"`  // or null
	}
}

type Session struct {
	UUID          string `json:"uuid"`
	Product_id    string `json:"product_id"`
	CreatedOn     int64  `json:"created_on"`
	FinishedOn    int64  `json:"finished_on,omitempty"`
	Status        string `json:"status"`
	Creator_ip    string `json:"creator_ip"`
	Abort_comment string `json:"abort_comment"` //or null
	Score         int64  `json:"score"`         //or null
	ScoreReason   int64  `json:"score_reason"`  //or null
	Comment       string `json:"score_text"`    //or null
	Billing_type  string `json:"billing_type"`  // or null
}

type SessionManager struct {
	Sessions []Session `json:"sessions"`
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
			var sm SessionManager
			err = json.Unmarshal([]byte(responseString), &sm)
			if err != nil {
				log.Printf("[ERROR] Ошибка парсинга: %v\n", err)
			}
			session := sm.Sessions[0]
			Session_ID = session.UUID
			log.Printf("[INFO] Подключение %s, billing: %s\n", session.Creator_ip, session.Billing_type)
			game, _ := readConfig(session.Product_id, fileGames)
			sessionOn, _ := dateTimeS(session.CreatedOn)
			ipInfo = ""

			if OnlineIpInfo {
				ipInfo = session.Creator_ip + onlineDBip(session.Creator_ip)
			} else {
				ipInfo = session.Creator_ip + offlineDBip(session.Creator_ip)
			}
			var billing string
			billing = session.Billing_type
			if billing != "" && billing != "trial" {
				billing = session.Billing_type
			}

			if TrialON {
				if billing == "trial" {
					sumTrial = getValueByKey(session.Creator_ip)
					if sumTrial == -1 { // нет записей по этому IP
						createOrUpdateKeyValue(session.Creator_ip, 0)
						billing = session.Billing_type
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
							log.Printf("[INFO] Заблокировано соединение: %s. Trial %d", session.Creator_ip, sumTrial)
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

		var sessionDur string
		var minute int

		foundSession, sessionDur, minute := getTimeStopComment(Session_ID, "Stop")

		billing := foundSession.Billing_type
		if sessionDur != "off" {
			var billingTrial string = ""
			if TrialON {
				if billing == "trial" {
					sumTrial = getValueByKey(foundSession.Creator_ip)
					if sumTrial < 20 || !TrialBlock {
						ipTrial := foundSession.Creator_ip
						handshake := foundSession.Abort_comment
						if !strings.Contains(handshake, "handshake") { // если кнопка "Играть тут" активированна, добавляем время в файл
							createOrUpdateKeyValue(ipTrial, minute)
						}
						sumTrial = getValueByKey(foundSession.Creator_ip)
						billingTrial = fmt.Sprintf("\nTrial %dмин", sumTrial)
					} else if sumTrial > 20 && TrialBlock {
						billingTrial = fmt.Sprintf("\nKICK - Trial %dмин", sumTrial)
					}
				}
			}
			var comment string
			if foundSession.Abort_comment != "" {
				comment = " - " + foundSession.Abort_comment
			}

			infoString = "\n" + sessionDur + comment + billingTrial
		} else {
			infoString = "off"
		}

	} else if status == "Comment" { // проверяем написание коммента
		var game, scoreReason string

		foundSession, sessionDur, _ := getTimeStopComment(Session_ID, "Comment")
		if foundSession.Comment == "" {
			time.Sleep(10 * time.Second)
		} else {
			log.Printf("[INFO] Отключение %s\n", foundSession.Creator_ip)
			game, _ = readConfig(foundSession.Product_id, fileGames)
			score := "Оценка: " + strconv.FormatInt(foundSession.Score, 10) + "\n"
			switch foundSession.ScoreReason {
			case 0:
				scoreReason = "Все ОК"
			case 1:
				scoreReason = "Игра не запустилась"
			case 2:
				scoreReason = "Лаги"
			case 3:
				scoreReason = "Тех. проблемы"
			case 4:
				scoreReason = "Обновления"
			default:
				scoreReason = "неизвестно"
			}

			if OnlineIpInfo {
				ipInfo = foundSession.Creator_ip + onlineDBip(foundSession.Creator_ip)
			} else {
				ipInfo = foundSession.Creator_ip + offlineDBip(foundSession.Creator_ip)
			}
			var billing string
			billing = foundSession.Billing_type
			if billing != "" && billing != "trial" {
				billing = foundSession.Billing_type
			}
			sessionOn, _ := dateTimeS(foundSession.CreatedOn)

			headText := hname + "<b><i>" + game + "</i></b>" + "\n" + ipInfo + "\n" + sessionOn + " | " + sessionDur + "\n" + billing
			log.Printf("[INFO] Получение комментария %s\n, %s ", foundSession.Creator_ip, foundSession.UUID)
			infoString = headText + "\n" + score + scoreReason + "\n" + foundSession.Comment
			//infoString = hname + "<b><i>" + game + "</i></b>" + "\n" + foundSession.Creator_ip + " - " + sessionDur + "\n" + foundSession.Comment
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

func getTimeStopComment(targetUUID, searchData string) (foundSession *Session, sessionDur string, minute int) {

	time.Sleep(5 * time.Second)
	for i := 0; i < 10; i++ {
		responseString, err := getFromURL(UrlSessions, "server_id", serverID)
		if err != nil {
			log.Println("[ERROR] Stop. Невозможно получить данные с сайта")
			time.Sleep(5 * time.Second)
		} else {
			// Парсим JSON
			var sm SessionManager
			err = json.Unmarshal([]byte(responseString), &sm)
			if err != nil {
				log.Printf("[ERROR] Ошибка парсинга: %v\n", err)
			}

			if searchData == "Stop" {
				// Ищем сессию по UUID
				for _, session := range sm.Sessions {
					if session.UUID == targetUUID {
						foundSession = &session
						break
					}
				}

				if foundSession != nil {
					if foundSession.FinishedOn != 0 {
						i = 10
					} else {
						time.Sleep(5 * time.Second)
					}
				}

			} else if searchData == "Comment" {
				// Ищем сессию по UUID
				for _, session := range sm.Sessions {
					if session.UUID == targetUUID {
						foundSession = &session
						break
					}
				}

				if foundSession.Comment == "" {
					time.Sleep(30 * time.Second)
				} else {
					i = 10
				}
			}
		}
	}

	_, stopTime := dateTimeS(foundSession.FinishedOn)
	_, startTime := dateTimeS(foundSession.CreatedOn)
	sessionDur, minute = dur(stopTime, startTime)

	return
}
