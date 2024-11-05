package main

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
    "github.com/gorilla/websocket"
    "log"
    "net/http"
    "sync"
)

var clients = make(map[*websocket.Conn]bool)
var mu sync.Mutex


func ProduceMessage(message string) {
	producer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": "localhost:9092"})
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	topic := "chat"

	msg := kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value: []byte(message),
	}
	producer.Produce(&msg, nil)
	producer.Flush(15 * 1000)
}


func HandleMessages() {
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": "localhost:9092",
		"group.id": "chat_group",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	consumer.Subscribe("chat", nil)

	for {
		msg, err := consumer.ReadMessage(-1)
		if err == nil {
			mu.Lock()
			for client := range clients {
				err := client.WriteMessage(websocket.TextMessage, msg.Value)
				if err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
					client.Close()
					delete(clients, client)
				}
			}
			mu.Unlock()
		} else {
			log.Printf("Ошибка: %v", err)
		}
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		log.Printf("Ошибка при попытке апгрейда: %v", err)
		return
	}

	mu.Lock()
	clients[conn] = true
	mu.Unlock()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Ошибка при чтении сообщения: %v", err)
			break
		}
		ProduceMessage(string(msg))
	}
	mu.Lock()
	delete(clients, conn)
	mu.Unlock()
}

func main() {
	go HandleMessages()
	http.HandleFunc("/ws", handleWebSocket)
	log.Fatal(http.ListenAndServe(":8081", nil))
}