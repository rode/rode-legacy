package main
import (
  "fmt"
  "time"
  "encoding/json"
  "github.com/nats-io/nats.go"
)
type Order struct {
	ID       string `json:"id"`
	URI      string `json:"image"`
	Advisory string `json:"advisory"`
}
func panicIfError(err error) {
	if err == nil {
		return
	}
	panic(err)
}
func processNextMessage(nc *nats.Conn) error {
	// Load the next message from the IMAGES stream BW consumer
	msg, err := nc.Request("$JS.API.CONSUMER.MSG.NEXT.ORDERS.DISPATCH", []byte(""), time.Minute)
	// ErrTimeout means there is no new message now lets just skip
	if err == nats.ErrTimeout {
		return nil
	}
	if err != nil {
		return err
	}
	// Parse the job
  order := &Order{}
  err = json.Unmarshal(msg.Data, order)
  if err != nil {
    return err
  }
  fmt.Printf("Processing %#v", order)
	// Acknowledge the message in JetStream, this will delete it from the work queue
	return msg.Respond(nil)
}
func main() {
  nc, err := nats.Connect("http://localhost:4222",nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
		panic(err)
	}))
  panicIfError(err)
  for {
    err = processNextMessage(nc)
  }
}
