package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
)

const magicGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func main() {
	http.HandleFunc("/ws", wsHandler)

	fmt.Println("Сервер запущен на :12345")
	err := http.ListenAndServe(":12345", nil)
	if err != nil {
		fmt.Println("Ошибка:", err)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "Expected websocket upgrade", http.StatusBadRequest)
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "Missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}

	// Формируем Sec-WebSocket-Accept
	h := sha1.New()
	h.Write([]byte(key + magicGUID))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Формируем ответ на handshake
	headers := http.Header{}
	headers.Add("Upgrade", "websocket")
	headers.Add("Connection", "Upgrade")
	headers.Add("Sec-WebSocket-Accept", acceptKey)

	// Отправляем статус 101 Switching Protocols
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	response := "HTTP/1.1 101 Switching Protocols\r\n"
	for k, v := range headers {
		response += fmt.Sprintf("%s: %s\r\n", k, v[0])
	}
	response += "\r\n"

	_, err = bufrw.WriteString(response)
	if err != nil {
		return
	}
	err = bufrw.Flush()
	if err != nil {
		return
	}

	// Теперь читаем и пишем фреймы WebSocket вручную
	for {
		msg, err := readTextFrame(bufrw.Reader)
		if err != nil {
			fmt.Println("Ошибка чтения:", err)
			break
		}
		fmt.Println("Получено сообщение:", msg)

		// Отправляем обратно
		err = writeTextFrame(bufrw.Writer, "echo: "+msg)
		if err != nil {
			fmt.Println("Ошибка записи:", err)
			break
		}
		err = bufrw.Flush()
		if err != nil {
			break
		}
	}
}

// Читаем простой текстовый вебсокет фрейм (без учёта опций маски и фрагментов)
func readTextFrame(r *bufio.Reader) (string, error) {
	// читаем первый байт (фин.фрейм + тип)
	b1, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	fin := b1&(1<<7) != 0
	opcode := b1 & 0x0F
	if opcode != 1 {
		return "", fmt.Errorf("только текстовые сообщения поддерживаются")
	}
	if !fin {
		return "", fmt.Errorf("фрагментированные сообщения не поддерживаются")
	}

	// читаем второй байт: маскирование + длина
	b2, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	mask := b2&(1<<7) != 0
	payloadLen := int(b2 & 0x7F)

	if payloadLen == 126 {
		b3, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		b4, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		payloadLen = int(b3)<<8 | int(b4)
	} else if payloadLen == 127 {
		return "", fmt.Errorf("большие сообщения не поддерживаются")
	}

	var maskingKey []byte
	if mask {
		maskingKey = make([]byte, 4)
		_, err := r.Read(maskingKey)
		if err != nil {
			return "", err
		}
	}

	payload := make([]byte, payloadLen)
	_, err = r.Read(payload)
	if err != nil {
		return "", err
	}

	if mask {
		for i := 0; i < payloadLen; i++ {
			payload[i] ^= maskingKey[i%4]
		}
	}

	return string(payload), nil
}

func writeTextFrame(w *bufio.Writer, msg string) error {
	payload := []byte(msg)
	header := []byte{0x81} // FIN=1, opcode=1

	// длина payload
	if len(payload) <= 125 {
		header = append(header, byte(len(payload)))
	} else if len(payload) <= 65535 {
		header = append(header, 126, byte(len(payload)>>8), byte(len(payload)&0xFF))
	} else {
		// большие сообщения не поддерживаются для простоты
		return fmt.Errorf("слишком большое сообщение")
	}

	_, err := w.Write(header)
	if err != nil {
		return err
	}
	_, err = w.Write(payload)
	if err != nil {
		return err
	}
	return nil
}
