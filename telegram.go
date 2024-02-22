package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// отправка сообщения ботом
func SendMessage(botToken string, chatID int64, text string, mesID int) (messageID int, err error) {
	var i int = 0

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Println("[ERROR] Ошибка подключения бота: ", err, getLine())
		return 0, err
	}
	bot.Debug = true

	if mesID != 0 {
		messageIDstr := fmt.Sprint(mesID)
		chatIDstr := fmt.Sprint(chatID)
		err := delMessage(chatIDstr, messageIDstr)
		// msg := tgbotapi.NewDeleteMessage(chatID, messageID)
		// _, err := bot.Send(msg)
		// err := deleteMessage(chatID, messageID)
		if err != nil {
			log.Println("[ERROR] Ошибка удаления сообщения", err, getLine())
		}
	}

	i = 0
	message := tgbotapi.NewMessage(chatID, text)
	message.ParseMode = "HTML"
	for i = 0; i < 3; i++ {
		sentMsg, err := bot.Send(message)
		if err != nil {
			log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
			time.Sleep(1 * time.Second)
			return 0, err
		} else if err == nil {
			messageID = sentMsg.MessageID
			i = 3
		}
	}

	return messageID, nil
}

func delMessage(chatID, messageID string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/deleteMessage", BotToken)

	requestBody, err := json.Marshal(map[string]string{
		"chat_id":    chatID,
		"message_id": messageID,
	})
	if err != nil {
		log.Println(err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Println(err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)

	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Println("Message deleted successfully!")
	} else {
		log.Println("Failed to delete message. Status code:", resp.StatusCode)
	}
	return err
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
						_, err := SendMessage(BotToken, userID, message, 0)
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
						_, err := SendMessage(BotToken, userID, chatMessage, 0) // отправка сообщения
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
									_, err := SendMessage(BotToken, userID, chatMessage, 0) // отправка сообщения
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

						_, err := SendMessage(BotToken, userID, messageText, 0)
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
							_, err = SendMessage(BotToken, userID, message, 0)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						} else {
							log.Printf("Станция %s в сети\n", hostname)
							message := fmt.Sprintf("Станция %s видна клиентам", hostname)
							_, err = SendMessage(BotToken, userID, message, 0)
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
							_, err = SendMessage(BotToken, userID, message, 0)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						} else {
							log.Printf("Станция %s спрятана\n", hostname)
							message := fmt.Sprintf("Станция %s спрятана от клиентов", hostname)
							_, err = SendMessage(BotToken, userID, message, 0)
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
					_, err := SendMessage(BotToken, userID, message, 0)
					if err != nil {
						log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
						return
					}
				} else if strings.Contains(messageT, "/delayreboot") {
					if strings.Contains(messageT, honame) { // Проверяем, что в тексте упоминается имя ПК
						go delayReboot(0)
						message := fmt.Sprintf("Будет выполнена перезагрузка %sпо окончании сессии", hname)
						_, err := SendMessage(BotToken, userID, message, 0)
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
							_, err := SendMessage(BotToken, userID, message, 0)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						} else {
							message := fmt.Sprintf("%sЗадача Streaming Service остановлена", hname)
							_, err := SendMessage(BotToken, userID, message, 0)
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
							_, err := SendMessage(BotToken, userID, message, 0)
							if err != nil {
								log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
								return
							}
						} else {
							message := fmt.Sprintf("%sЗадача Streaming Service запущена", hname)
							_, err := SendMessage(BotToken, userID, message, 0)
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

					_, err := SendMessage(BotToken, userID, message, 0)
					if err != nil {
						log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
						return
					}
				} else {
					messageText := "Неизвестная команда"
					_, err := SendMessage(BotToken, userID, messageText, 0)
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
