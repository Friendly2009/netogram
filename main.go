package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

type Client struct {
	conn net.Conn
	nick string
}

var (
	clients  = make(map[net.Conn]*Client)
	messages = make(chan string)
	mu       sync.Mutex            // для безопасности доступа к clients
	ln       net.Listener          // слушатель
	done     = make(chan struct{}) // канал для выключения сервера
)

func main() {
	var err error
	ln, err = net.Listen("tcp", ":12345")
	if err != nil {
		fmt.Println("Ошибка запуска сервера:", err)
		return
	}
	defer ln.Close()

	go broadcastMessages()

	fmt.Println("Мессенджер запущен на порту 12345")
	go waitForExit() // горутина для ожидания команды выключения

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-done:
				// Сервер закрыт, выходим из цикла
				fmt.Println("Сервер выключен")
				return
			default:
				fmt.Println("Ошибка подключения:", err)
				continue
			}
		}

		client := &Client{conn: conn, nick: "Аноним"}

		mu.Lock()
		clients[conn] = client
		mu.Unlock()

		go handleClient(client)
	}
}

func waitForExit() {
	reader := bufio.NewReader(os.Stdin)
	for {
		input, _ := reader.ReadString('\n')
		if strings.TrimSpace(input) == "exit" {
			fmt.Println("Выключаем сервер...")

			// Закрываем слушатель, чтобы остановить Accept()
			ln.Close()

			// Закрываем все подключения
			mu.Lock()
			for _, c := range clients {
				c.conn.Close()
			}
			mu.Unlock()

			close(done) // сообщаем о закрытии
			os.Exit(0)  // завершаем программу
		}
	}
}

func handleClient(client *Client) {
	defer func() {
		mu.Lock()
		delete(clients, client.conn)
		mu.Unlock()
		client.conn.Close()
	}()

	reader := bufio.NewReader(client.conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		message = strings.TrimSpace(message)

		if message == "" {
			continue
		}

		if strings.HasPrefix(message, "/nick ") {
			newNick := strings.TrimSpace(strings.TrimPrefix(message, "/nick "))
			if newNick != "" {
				oldNick := client.nick
				client.nick = newNick
				messages <- fmt.Sprintf("Теперь ник %s — %s", oldNick, newNick)
			}
		} else {
			messages <- fmt.Sprintf("[%s] %s", client.nick, message)
		}
	}
}

func broadcastMessages() {
	for msg := range messages {
		mu.Lock()
		for _, c := range clients {
			_, err := fmt.Fprintln(c.conn, msg)
			if err != nil {
				c.conn.Close()
				delete(clients, c.conn)
			}
		}
		mu.Unlock()
	}
}
