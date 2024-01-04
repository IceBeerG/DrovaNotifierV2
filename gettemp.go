package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Node struct {
	ID       int    `json:"id"`
	Text     string `json:"Text"`
	Min      string `json:"Min"`
	Value    string `json:"Value"`
	Max      string `json:"Max"`
	ImageURL string `json:"ImageURL"`
	Children []Node `json:"Children"`
}

func CheckHWt(hostname string) {
	var xCPU, xGPU, xGPUf bool = false, false, false
	for {
		var fanZ float64
		tCPU, tGPU, tGPUhs, fan1, fanp1, fan2, fanp2, _ := GetTemperature()
		if fan2 == -1 {
			fanZ = fan1
		} else {
			fanZ = fan2
		}

		if tGPUhs == -1 {
			tGPUhs = tGPU
		}

		if tCPU > CPUtmax && !xCPU {
			var tCPUsum float64 = 0
			for i := 0; i < 6; i++ {
				tCPU, _, _, _, _, _, _, _ = GetTemperature()
				tCPUsum += tCPU
				time.Sleep(5 * time.Second)
			}
			log.Printf("tCPUsum = %.1f\n", tCPUsum)
			tCPUavg := tCPUsum / 6
			if tCPUavg > CPUtmax {
				chatMessage := fmt.Sprintf("Внимание! "+hostname+"\nt CPU: %.1f °C\nt CPU avg:  %.1f °C", tCPU, tCPUavg)
				err := SendMessage(BotToken, Chat_IDint, chatMessage)
				if err != nil {
					log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
				}
				xCPU = true
			}
		} else if xCPU {
			var tCPUsum float64 = 0
			for i := 0; i < 6; i++ {
				tCPU, _, _, _, _, _, _, _ = GetTemperature()
				tCPUsum += tCPU
				time.Sleep(5 * time.Second)
			}
			log.Printf("tCPUsum = %.1f\n", tCPUsum)
			tCPUavg := tCPUsum / 6
			if tCPUavg < CPUtmax-DeltaT {
				chatMessage := fmt.Sprintf("Норма. "+hostname+"\nt CPU: %.1f °C\nt CPU avg:  %.1f °C", tCPU, tCPUavg)
				err := SendMessage(BotToken, Chat_IDint, chatMessage)
				if err != nil {
					log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
				}
				xCPU = false
			}
		}

		if (tGPU > FANt || tGPUhs > FANt) && (fan1 < FANrpm || fanZ < FANrpm) && !xGPUf {
			var chatMessage string
			time.Sleep(2 * time.Second)
			_, tGPU, tGPUhs, fan1, fanp1, fan2, fanp2, _ := GetTemperature()
			if fan2 == -1 {
				fanZ = fan1
			} else {
				fanZ = fan2
			}

			if tGPUhs == -1 {
				tGPUhs = tGPU
			}
			if (tGPU > FANt || tGPUhs > FANt) && (fan1 < FANrpm || fanZ < FANrpm) {
				chatMessage = fmt.Sprintf("Внимание! "+hostname+"\nt GPU: %.1f °C\nt HotSpot: %.1f °C\n", tGPU, tGPUhs)
				chatMessage += fmt.Sprintf("Fan1: %.0f RPM - %.0f %%\n", fan1, fanp1)
				if fan2 != -1 {
					chatMessage += fmt.Sprintf("Fan2: %.0f RPM - %.0f %%\n", fan2, fanp2)
				}
				err := SendMessage(BotToken, Chat_IDint, chatMessage)
				if err != nil {
					log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
				}
				xGPUf = true
			}
		} else if xGPUf && tGPU > FANt && tGPUhs > FANt && fan1 > FANrpm && fanZ > FANrpm {
			var chatMessage string
			chatMessage = fmt.Sprintf("Норма. "+hostname+"\nt GPU: %.1f °C\nt HotSpot: %.1f °C\n", tGPU, tGPUhs)
			chatMessage += fmt.Sprintf("Fan1: %.0f RPM - %.0f %%\n", fan1, fanp1)
			if fan2 != -1 {
				chatMessage += fmt.Sprintf("Fan2: %.0f RPM - %.0f %%\n", fan2, fanp2)
			}
			err := SendMessage(BotToken, Chat_IDint, chatMessage)
			if err != nil {
				log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
			}
			xGPUf = false
		}

		if (tGPU > GPUtmax || tGPUhs > GPUhsTmax) && !xGPU {
			var chatMessage string
			var tGPUsum, tGPUhsSum float64 = 0, 0
			log.Printf("tGPU = %.1f, tGPUhs = %.1f\n", tGPU, tGPUhs)
			for i := 0; i < 6; i++ {
				_, tGPU, tGPUhs, _, _, _, _, _ = GetTemperature()
				tGPUsum += tGPU
				tGPUhsSum += tGPUhs
				time.Sleep(5 * time.Second)
			}
			log.Printf("tGPUsum = %.1f\n", tGPUsum)
			log.Printf("tGPUhsSum = %.1f\n", tGPUhsSum)
			tGPUavg := tGPUsum / 6
			tGPUhsAvg := tGPUhsSum / 6
			if tGPUavg > GPUtmax || tGPUhsAvg > GPUhsTmax {
				log.Printf("tGPUavg = %.1f, tGPUhsAvg = %.1f\n", tGPUavg, tGPUhsAvg)
				chatMessage = fmt.Sprintf("Внимание! "+hostname+"\nt GPU: %.1f °C\nt HotSpot: %.1f °C\n", tGPU, tGPUhs)
				if fan1 != -1 {
					chatMessage += fmt.Sprintf("Fan1: %.0f RPM - %.0f %%\n", fan1, fanp1)
					if fan2 != -1 {
						chatMessage += fmt.Sprintf("Fan2: %.0f RPM - %.0f %%\n", fan2, fanp2)
					}
				}
				err := SendMessage(BotToken, Chat_IDint, chatMessage)
				if err != nil {
					log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
				}
				xGPU = true
			}
		} else if tGPU < GPUtmax && tGPUhs < GPUhsTmax && xGPU {
			var chatMessage string
			var tGPUsum, tGPUhsSum float64 = 0, 0
			log.Printf("tGPU = %.1f, tGPUhs = %.1f\n", tGPU, tGPUhs)
			for i := 0; i < 6; i++ {
				_, tGPU, tGPUhs, _, _, _, _, _ = GetTemperature()
				tGPUsum += tGPU
				tGPUhsSum += tGPUhs
				time.Sleep(5 * time.Second)
			}
			log.Printf("tGPUsum = %.1f\n", tGPUsum)
			log.Printf("tGPUhsSum = %.1f\n", tGPUhsSum)
			tGPUavg := tGPUsum / 6
			tGPUhsAvg := tGPUhsSum / 6
			if tGPUavg < GPUtmax-DeltaT && tGPUhsAvg < GPUhsTmax-DeltaT {
				log.Printf("tGPUavg = %.1f, tGPUhsAvg = %.1f\n", tGPUavg, tGPUhsAvg)
				chatMessage = fmt.Sprintf("Норма. "+hostname+"\nt GPU: %.1f °C\nt HotSpot: %.1f °C\n", tGPU, tGPUhs)
				if fan1 != -1 {
					chatMessage += fmt.Sprintf("Fan1: %.0f RPM - %.0f %%\n", fan1, fanp1)
					if fan2 != -1 {
						chatMessage += fmt.Sprintf("Fan2: %.0f RPM - %.0f %%\n", fan2, fanp2)
					}
				}
				err := SendMessage(BotToken, Chat_IDint, chatMessage)
				if err != nil {
					log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
				}
				xGPU = false
			}
		}
		time.Sleep(1 * time.Minute)
	}
}

func GetTemperature() (tCPU, tGPU, tGPUhs, fan1, fanp1, fan2, fanp2 float64, tMessage string) {
	var body []byte
	tMessage = ""
	urlLHM := "http://localhost:8085/data.json"
	respLHM, err := http.Get(urlLHM)
	if err != nil {
		log.Println(err)
		restart()
	}
	defer respLHM.Body.Close()

	body, err = io.ReadAll(respLHM.Body)
	if err != nil {
		log.Println(err)
	}

	tCPU, tGPU, tGPUhs, fan1, fanp1, fan2, fanp2 = -1, -1, -1, -1, -1, -1, -1

	tempCPU1, tempCPU2 := getTemp(body, "cpu")
	if tempCPU1 != "-1" {
		tMessage += fmt.Sprintf("t CPU = %s\n", tempCPU1)
		tCPU = takeFloat(tempCPU1)
	} else if tempCPU2 != "-1" {
		tMessage += fmt.Sprintf("t CPU = %s\n", tempCPU2)
		tCPU = takeFloat(tempCPU2)
	}

	tempGPUnv1, tempGPUnv2 := getTemp(body, "gpuNVidia")

	if tempGPUnv1 != "-1" {
		tMessage += fmt.Sprintf("t GPU = %s\n", tempGPUnv1)
		tGPU = takeFloat(tempGPUnv1)

		if tempGPUnv2 != "-1" {
			tMessage += fmt.Sprintf("t GPU HotSpot= %s\n", tempGPUnv2)
			tGPUhs = takeFloat(tempGPUnv2)
		}

		fanNV1, fanNV2 := getTemp(body, "fanNVidia")
		if fanNV1 != "-1" {
			fanNVp1, _ := getTemp(body, "fanNVp")
			tMessage += fmt.Sprintf("t GPU fan1 = %s\nGPU fan1 = %s\n", fanNV1, fanNVp1)
			fan1 = takeFloat(fanNV1)
			fanp1 = takeFloat(fanNVp1)
		}
		if fanNV2 != "-1" {
			_, fanNVp2 := getTemp(body, "fanNVp")
			tMessage += fmt.Sprintf("t GPU fan2 = %s\nGPU fan2 = %s\n", fanNV2, fanNVp2)
			fan2 = takeFloat(fanNV2)
			fanp2 = takeFloat(fanNVp2)
		}
	} else {
		tempGPUa1, tempGPUa2 := getTemp(body, "gpuAMD")
		if tempGPUa1 != "-1" {
			tMessage += fmt.Sprintf("t GPU = %s\n", tempGPUa1)
			tGPU = takeFloat(tempGPUa1)
			if tempGPUa2 != "-1" {
				tMessage += fmt.Sprintf("t GPU HotSpot= %s\n", tempGPUa2)
				tGPUhs = takeFloat(tempGPUa2)
			}

			fanA1, fanA2 := getTemp(body, "fanAMD")
			if fanA1 != "-1" {
				tMessage += fmt.Sprintf("t GPU fan1 = %s\n", fanA1)
				fan1 = takeFloat(fanA1)
			}
			if fanA2 != "-1" {
				tMessage += fmt.Sprintf("t GPU fan2 = %s\n", fanA2)
				fan2 = takeFloat(fanA2)
			}

		} else {
			tempGPUi1, tempGPUi2 := getTemp(body, "gpuINTEL")
			if tempGPUi1 != "-1" {
				tMessage += fmt.Sprintf("t GPU = %s\n", tempGPUi1)
				tGPU = takeFloat(tempGPUi1)

				if tempGPUi2 != "-1" {
					tMessage += fmt.Sprintf("t GPU HotSpot= %s\n", tempGPUi2)
					tGPUhs = takeFloat(tempGPUi2)
				}

				fanI1, fanI2 := getTemp(body, "fanINTEL")
				if fanI1 != "-1" {
					tMessage += fmt.Sprintf("t GPU fan1 = %s\n", fanI1)
					fan1 = takeFloat(fanI1)
				}
				if fanI2 != "-1" {
					tMessage += fmt.Sprintf("t GPU fan2 = %s\n", fanI2)
					fan2 = takeFloat(fanI2)
				}
			}
		}
	}
	return
}

func takeFloat(valueS string) (valueF float64) {
	valueS = strings.TrimSuffix(valueS, " °C")
	valueS = strings.TrimSuffix(valueS, " RPM")
	valueS = strings.TrimSuffix(valueS, " %")
	valueF, err := strconv.ParseFloat(strings.ReplaceAll(valueS, ",", "."), 64)
	if err != nil {
		fmt.Println("Ошибка конвертирования. ", err)
	}
	return valueF
}

func getTemp(body []byte, xpu string) (value1, value2 string) {

	var root Node
	var text1, text2, text3, text4 string
	err := json.Unmarshal(body, &root)
	if err != nil {
		fmt.Println(err)
	}

	value1, value2 = "-1", "-1"

	if xpu == "cpu" {
		text1 = "images_icon/cpu.png"
		text4 = "Temperatures"
		text2 = "Core (Tctl/Tdie)"
		text3 = "CPU Package"
	} else if xpu == "gpuNVidia" {
		text1 = "images_icon/nvidia.png"
		text4 = "Temperatures"
		text2 = "GPU Core"
		text3 = "GPU Hot Spot"
	} else if xpu == "fanNVidia" {
		text1 = "images_icon/nvidia.png"
		text4 = "Fans"
		text2 = "GPU Fan 1"
		text3 = "GPU Fan 2"
	} else if xpu == "gpuAMD" {
		text1 = "images_icon/amd.png"
		text4 = "Temperatures"
		text2 = "GPU Core"
		text3 = "GPU Hot Spot"
	} else if xpu == "fanAMD" {
		text1 = "images_icon/amd.png"
		text4 = "Fans"
		text2 = "GPU Fan 1"
		text3 = "GPU Fan 2"
	} else if xpu == "gpuINTEL" {
		text1 = "images_icon/intel.png"
		text4 = "Temperatures"
		text2 = "GPU Core"
		text3 = "GPU Hot Spot"
	} else if xpu == "fanINTEL" {
		text1 = "images_icon/intel.png"
		text4 = "Fans"
		text2 = "GPU Fan 1"
		text3 = "GPU Fan 2"
	} else if xpu == "fanNVp" {
		text1 = "images_icon/nvidia.png"
		text4 = "Controls"
		text2 = "GPU Fan 1"
		text3 = "GPU Fan 2"
	}

	// Перебор первого уровня
	for _, child := range root.Children {
		// Перебор второго уровня
		for _, subChild := range child.Children {
			if subChild.ImageURL == text1 {
				// Перебор третьего уровня
				for _, subSubChild := range subChild.Children {
					if subSubChild.Text == text4 {
						// Перебор четвертого уровня
						for _, subSubSubChild := range subSubChild.Children {
							if subSubSubChild.Text == text2 || (subSubSubChild.Text == "GPU Fan" && text1 == "images_icon/nvidia.png") {
								value1 = subSubSubChild.Value
							}
							if subSubSubChild.Text == text3 {
								value2 = subSubSubChild.Value
							}
						}
					}
				}
			}
		}
	}
	return value1, value2
}

// func CheckSSDtemp() {
// 	var body []byte
// 	// tMessage = ""
// 	urlLHM := "http://localhost:8085/data.json"
// 	respLHM, err := http.Get(urlLHM)
// 	if err != nil {
// 		log.Println(err)
// 		restart()
// 	}
// 	defer respLHM.Body.Close()

// 	body, err = io.ReadAll(respLHM.Body)
// 	if err != nil {
// 		log.Println(err)
// 	}

// 	tempSSD1, tempSSD2 := GetTemperatureDisk(body)
// 	if tempSSD1 != "" {
// 		fmt.Println("t1 disk = ", tempSSD1)
// 	}
// 	if tempSSD2 != "" {
// 		fmt.Println("t2 disk = ", tempSSD2)
// 	}
// }

// func GetTemperatureDisk(body []byte) (value1, value2 string) {
// 	var root Node
// 	var text1, text2, text3, text4 string
// 	err := json.Unmarshal(body, &root)
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// 	text1 = "images_icon/hdd.png"
// 	text4 = "Temperatures"
// 	text2 = "Temperature"
// 	text3 = "Temperature 2"

// 	value1, value2 = "-1", "-1"
// 	// Перебор первого уровня
// 	for _, child := range root.Children {
// 		// Перебор второго уровня
// 		for _, subChild := range child.Children {
// 			// fmt.Println("child.Children - ", subChild.ImageURL)
// 			if subChild.ImageURL == text1 {
// 				// Перебор третьего уровня
// 				for _, subSubChild := range subChild.Children {
// 					if subSubChild.Text == text4 {
// 						// Перебор четвертого уровня
// 						for _, subSubSubChild := range subSubChild.Children {
// 							if subSubSubChild.Text == text2 {
// 								if value1 == "" {
// 									value1 = subSubSubChild.Value
// 								} else {
// 									value1 += ", " + subSubSubChild.Value
// 								}
// 							}
// 							if subSubSubChild.Text == text3 {
// 								if value2 == "" {
// 									value2 = subSubSubChild.Value
// 								} else {
// 									value2 += ", " + subSubSubChild.Value
// 								}
// 							}
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// 	return value1, value2
// }
