package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/shirou/gopsutil/net"
)

type Release struct {
	PublishedAt time.Time `json:"published_at"`
}

func getSpeed() (string, float64) {
	var maxInterfaceName string
	var maxOutgoingSpeed float64

	r1, _ := net.IOCounters(true) // получение информации об интерфейсах
	time.Sleep(15 * time.Second)
	r2, _ := net.IOCounters(true) // получение информации об интерфейсах через 10 секунду

	// Поиск интерфейса с максимальной исходящей скоростью
	for _, r := range r2 {
		for _, r_1 := range r1 {
			if r.Name == r_1.Name {
				outgoingSpeed := float64(r.BytesSent - r_1.BytesSent)
				if outgoingSpeed > maxOutgoingSpeed {
					maxOutgoingSpeed = outgoingSpeed
					maxInterfaceName = r.Name
				}
			}
		}
	}
	return maxInterfaceName, maxOutgoingSpeed
}

func updateGeoLite(mmdbASN, mmdbCity string) {
	_, err := http.Get("https://github.com")
	if err != nil {
		log.Println("[ERROR] github.com недоступен. Обновление GeoLite невозможно")
	} else {
		asnURL := "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-ASN.mmdb"
		cityURL := "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-City.mmdb"
		z := downloadAndReplaceFileIfNeeded(asnURL, mmdbASN)
		z += downloadAndReplaceFileIfNeeded(cityURL, mmdbCity)
		if z > 0 {
			log.Println("[INFO] Перезапуск приложения")
			restart()
		}
	}
}

func downloadAndReplaceFileIfNeeded(url, filename string) int8 {
	var z int8 = 0
	time.Sleep(2 * time.Second)
	resp, err := http.Get("https://api.github.com/repos/P3TERX/GeoLite.mmdb/releases/latest")
	if err != nil {
		log.Println("[ERROR] Ошибка: ", err, getLine())
		restart()
	}
	defer resp.Body.Close()

	var release Release
	err = json.NewDecoder(resp.Body).Decode(&release)
	if err != nil {
		log.Println("[ERROR] Ошибка: ", err, getLine())
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		log.Println("[ERROR] Ошибка получения информации по файлу: ", err, getLine())
	}
	fileModTime := fileInfo.ModTime()

	if fileModTime.Before(release.PublishedAt) {
		// Отправка GET-запроса для загрузки файла
		resp, err := http.Get(url)
		if err != nil {
			log.Println("[ERROR] Ошибка отправки запроса: ", err, getLine())
		}
		defer resp.Body.Close()

		// Создание нового файла и копирование данных из тела ответа
		out, err := os.Create(filename)
		if err != nil {
			log.Println("[ERROR] Ошибка: ", err, getLine())
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			log.Println("[ERROR] Ошибка замены файлов: ", err, getLine())
		} else {
			log.Printf("[INFO] Файл %s обновлен\n", filename)
			z++
		}
	} else {
		log.Printf("[INFO] Файл %s уже обновлен\n", filename)
		z = 0
	}
	return z
}

// слежение за изменением файлов базы IP
func restartGeoLite(mmdbASN, mmdbCity string) {
	var geoLite = [2]string{mmdbASN, mmdbCity}
	var previousModTime = [2]time.Time{}

	for i := 0; i < len(geoLite); i++ {
		fileInfo, err := os.Stat(geoLite[i])
		if err != nil {
			log.Println(err, getLine())
		}
		// Проверяем время последнего изменения файла
		previousModTime[i] = fileInfo.ModTime()
	}

	for {
		for i := 0; i < len(geoLite); i++ {
			fileInfo, err := os.Stat(geoLite[i])
			if err != nil {
				log.Println(err, getLine())
			}
			if previousModTime[i] != fileInfo.ModTime() {
				log.Println("[INFO] Файл был изменен. Перезапуск приложения...")
				restart()
			}
		}
		time.Sleep(5 * time.Minute) // Интервал повторной проверки
	}
}

// инфо по IP - ipinfo.io
func onlineDBip(ip string) (text string) {
	_, err := http.Get("https://ipinfo.io")
	if err != nil {
		fmt.Println("Сайт ipinfo.io недоступен")
	} else {
		apiURL := fmt.Sprintf("https://ipinfo.io/%s/json", ip)
		resp, err := http.Get(apiURL)
		if err != nil {
			log.Println(err, getLine())
		}
		defer resp.Body.Close()

		var ipInfo IPInfoResponse
		err = json.NewDecoder(resp.Body).Decode(&ipInfo)
		if err != nil {
			log.Println(err, getLine())
		}

		var city, region, isp string = "", "", ""
		if ipInfo.City != "" {
			city = " - " + ipInfo.City
		}

		if ipInfo.Region != "" {
			region = " - " + ipInfo.Region
		}

		if ipInfo.ISP != "" {
			isp = " - " + ipInfo.ISP
		}

		text = city + region + isp
	}
	return text

}
