package main

import (
	"encoding/json"
	"fmt"
	"log"

	orderdlq "ecommerce-dead-letter-worker"
)

func main() {
	client, err := orderdlq.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	messages, err := client.Consume(10, 60)
	if err != nil {
		log.Fatal(err)
	}
	for _, message := range messages {
		if err := deliverOrder(message.Payload); err != nil {
			if err := client.PublishDeadLetter(message); err != nil {
				log.Printf("dead-letter %s: %v", message.MessageID, err)
				continue
			}
			if err := client.Ack(message.MessageID); err != nil {
				log.Printf("ack %s: %v", message.MessageID, err)
				continue
			}
			fmt.Printf("dead-lettered %s\n", message.MessageID)
			continue
		}
		if err := client.Ack(message.MessageID); err != nil {
			log.Printf("ack %s: %v", message.MessageID, err)
		}
	}
}

func deliverOrder(payload json.RawMessage) error {
	var order struct {
		OrderID string `json:"order_id"`
	}
	if err := json.Unmarshal(payload, &order); err != nil {
		return err
	}
	if order.OrderID == "" {
		return fmt.Errorf("order_id is required")
	}
	return nil
}
