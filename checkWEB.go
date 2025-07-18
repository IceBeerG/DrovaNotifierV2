package main

import (
	"fmt"
	"net/http"
	"time"
)

// Функция для проверки доступности сайта
func isSiteAvailable(url string) bool {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	_, err := client.Head(url) // Используем HEAD для быстрой проверки
	return err == nil
}

// Функция ожидания доступности сайта
func waitForSiteAvailableee(url string) {
	checkInterval := 5 * time.Second

	for i := 0; i < 10; i++ {
		if isSiteAvailable(url) {
			fmt.Printf("Сайт %s доступен\n", url)
			return
		}

		fmt.Printf("Ожидание доступности сайта %s...\n", url)
		time.Sleep(checkInterval)
	}
}
