package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

// Структура для хранения информации о каждом клиенте
type Client struct {
	conn net.Conn // Само сетевое соединение
	nick string   // Никнейм клиента
}

// В мапе храним клиентов: ключ — соединение, значение — структура Client
var clients = make(map[net.Conn]*Client)

// Канал для обмена сообщениями, предназначен для рассылки всем клиентам
var messages = make(chan string)

func main() {
	// Запускаем слушатель на порту 12345
	ln, err := net.Listen("tcp", ":12345")
	if err != nil {
		fmt.Println("Ошибка запуска сервера:", err)
		return
	}
	defer ln.Close()

	// Запускаем отдельную горутину для постоянной рассылки сообщений всем клиентам
	go broadcastMessages()

	fmt.Println("Мессенджер запущен на порту 12345")

	for {
		// Принимаем новые входящие подключения
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Ошибка подключения:", err)
			continue
		}

		// Создаем нового клиента с ником по умолчанию "Аноним"
		client := &Client{
			conn: conn,
			nick: "Аноним",
		}
		// Добавляем клиента в список
		clients[conn] = client
		// Обрабатываем клиента в отдельной горутине
		go handleClient(client)
	}
}

// Обработка каждого клиента
func handleClient(client *Client) {
	defer func() {
		// После отключения клиента удаляем его из списка и закрываем соединение
		delete(clients, client.conn)
		client.conn.Close()
	}()

	reader := bufio.NewReader(client.conn)

	for {
		// Читаем сообщение от клиента до символа новой строки
		message, err := reader.ReadString('\n')
		if err != nil {
			// Ошибка чтения (например, отключение) — выходим из цикла
			return
		}
		message = strings.TrimSpace(message)

		if message == "" {
			continue // игнорируем пустые сообщения
		}

		// Проверка, есть ли команда смены ника
		if strings.HasPrefix(message, "/nick ") {
			// Вытаскиваем новый ник из сообщения
			newNick := strings.TrimSpace(strings.TrimPrefix(message, "/nick "))
			if newNick != "" {
				// Сохраняем старый ник для уведомления
				oldNick := client.nick
				// Обновляем ник клиента
				client.nick = newNick
				// Отправляем системное сообщение о смене ника во все чаты
				systemMsg := fmt.Sprintf("Теперь ник %s — %s", oldNick, newNick)
				messages <- systemMsg
			}
		} else {
			// Обычное сообщение, добавляем никнейм клиента
			fullMsg := fmt.Sprintf("[%s] %s", client.nick, message)
			// Передаем сообщение для рассылки всем
			messages <- fullMsg
		}
	}
}

// Горутина для постоянной рассылки сообщений всем подключенным клиентам
func broadcastMessages() {
	for {
		// Получаем сообщение из канала
		msg := <-messages
		// Отправляем его каждому клиенту
		for _, c := range clients {
			_, err := fmt.Fprintln(c.conn, msg)
			if err != nil {
				// Если произошла ошибка, закрываем соединение и удаляем клиента
				c.conn.Close()
				delete(clients, c.conn)
			}
		}
	}
}
