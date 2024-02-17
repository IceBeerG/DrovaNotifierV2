package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/shirou/gopsutil/disk"
	"golang.org/x/sys/windows/registry"
)

type VideoController struct {
	Name          string
	DriverVersion string
}

type Win32NetworkAdapter struct {
	Name  string
	Speed uint64
}

// Получаем версию видеодрайвера
func videoDriver() (vDrv string) {
	var controllers []VideoController
	query := "SELECT Name, DriverVersion FROM Win32_VideoController"
	err := wmi.Query(query, &controllers)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	vDrv = ""
	for _, controller := range controllers {
		if !strings.Contains(controller.Name, "Drova Display") {
			vDrv += fmt.Sprintf("%s driver: %s\n", controller.Name, controller.DriverVersion)
		}
	}
	return
}

// Перезагрузка ПК
func rebootPC() {
	cmd := exec.Command("shutdown", "/r", "/t", "0")
	err := cmd.Run()
	if err != nil {
		log.Println(err)
		return
	}
}

// Проверка файлов античитов
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

// Проверяем свободное место на дисках
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

// Перезапуск приложения
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

// Получаем данные из реестра
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

// Проверяет, запущен ли указанный процесс
func checkIfProcessRunning(processName string) bool {
	cmd := exec.Command("tasklist")
	output, err := cmd.Output()
	if err != nil {
		log.Println("[ERROR] Ошибка получения списка процессов:", err, getLine())
	}
	return strings.Contains(string(output), processName)
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
		verDriver := videoDriver()
		if viewHostname {
			hname = hostname + " "
		}
		message := fmt.Sprintf("Станция %sзапущена менее 5 минут назад!\nВремя запуска - %s\n%s", hname, formattedTime, verDriver)
		err := SendMessage(BotToken, ServiceChatID, message)
		if err != nil {
			log.Println("[ERROR] Ошибка отправки сообщения: ", err, getLine())
		}
	}
}

func getLinkSpeed(interfaceName string) string {
	var adapters []Win32NetworkAdapter
	var linkSpeed uint64 = 0
	query := fmt.Sprintf("SELECT Name, Speed FROM Win32_NetworkAdapter WHERE NetConnectionID = '%s'", interfaceName)
	err := wmi.Query(query, &adapters)
	if err != nil {
		log.Printf("[ERROR] Ошибка получения скорости интерфейса %s через wmi\n", interfaceName)
	}

	if len(adapters) > 0 {
		linkSpeed = adapters[0].Speed

		if linkSpeed < 1000000000 {
			return fmt.Sprintf(" (%dM)", linkSpeed/1000000)
		} else {
			if linkSpeed == 2500000000 {
				return fmt.Sprintf(" (%.1fG)", float64(linkSpeed)/1000000000)
			} else {
				return fmt.Sprintf(" (%dG)", linkSpeed/1000000000)
			}
		}
	}
	return ""
}
