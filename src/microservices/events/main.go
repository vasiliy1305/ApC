package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/segmentio/kafka-go"
)

var (
    kafkaBroker = "kafka:9092"
    topicUser   = "user-events"
    topicPayment = "payment-events"
    topicMovie  = "movie-events"
)

type Event struct {
    Type string                 `json:"type"`
    Data map[string]interface{} `json:"data"`
}

func produceEvent(topic string, event Event) error {
    w := kafka.Writer{
        Addr:     kafka.TCP(kafkaBroker),
        Topic:    topic,
        Balancer: &kafka.LeastBytes{},
    }
    defer w.Close()

    msgBytes, err := json.Marshal(event)
    if err != nil {
        return err
    }

    return w.WriteMessages(context.Background(),
        kafka.Message{Value: msgBytes},
    )
}

func consumeEvents(topic string, stop chan struct{}) {
    r := kafka.NewReader(kafka.ReaderConfig{
        Brokers: []string{kafkaBroker},
        Topic:   topic,
        GroupID: "events-group",
    })
    defer r.Close()

    for {
        select {
        case <-stop:
            log.Printf("Stopping consumer for topic %s", topic)
            return
        default:
            m, err := r.ReadMessage(context.Background())
            if err != nil {
                log.Println("Error reading message:", err)
                continue
            }
            var e Event
            if err := json.Unmarshal(m.Value, &e); err != nil {
                log.Println("Error unmarshaling event:", err)
                continue
            }
            log.Printf("Consumed from %s: %+v\n", topic, e)
        }
    }
}

func main() {
    stop := make(chan struct{})

    go consumeEvents(topicUser, stop)
    go consumeEvents(topicPayment, stop)
    go consumeEvents(topicMovie, stop)

    http.HandleFunc("/api/events/user", func(w http.ResponseWriter, r *http.Request) {
        e := Event{Type: "user", Data: map[string]interface{}{"id": 123, "name": "John Doe"}}
        if err := produceEvent(topicUser, e); err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        w.WriteHeader(http.StatusCreated)
        w.Write([]byte(`{"status":"success"}`))
    })

    http.HandleFunc("/api/events/payment", func(w http.ResponseWriter, r *http.Request) {
        e := Event{Type: "payment", Data: map[string]interface{}{"id": 456, "amount": 99.99}}
        if err := produceEvent(topicPayment, e); err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        w.WriteHeader(http.StatusCreated)
        w.Write([]byte(`{"status":"success"}`))
    })

    http.HandleFunc("/api/events/movie", func(w http.ResponseWriter, r *http.Request) {
        e := Event{Type: "movie", Data: map[string]interface{}{"id": 789, "title": "New Movie"}}
        if err := produceEvent(topicMovie, e); err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        w.WriteHeader(http.StatusCreated)
        w.Write([]byte(`{"status":"success"}`))
    })

    http.HandleFunc("/api/events/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":true}`))
    })

    // Graceful shutdown
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sig
        close(stop)
        os.Exit(0)
    }()

    fmt.Println("Starting events-service on :8082")
    log.Fatal(http.ListenAndServe(":8082", nil))
}
