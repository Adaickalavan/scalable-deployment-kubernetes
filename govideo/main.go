package main

import (
	"confluentkafkago"
	"encoding/json"
	"fmt"
	"log"
	"mjpeg"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"gocv.io/x/gocv"
)

var (
	stream        *mjpeg.Stream
	broker        = os.Getenv("KAFKAPORT")
	topics        = []string{os.Getenv("TOPICNAME")}
	group         = os.Getenv("GROUPNAME")
	displayport   = os.Getenv("DISPLAYPORT")
	nodeport      = os.Getenv("NODEPORT")
	frameInterval = time.Duration(getenvint("FRAMEINTERVAL"))
)

func main() {
	// Create new mjpeg stream
	stream = mjpeg.NewStream(frameInterval)

	// Create new Consumer in a new ConsumerGroup
	c, err := confluentkafkago.NewConsumer(broker, group)
	if err != nil {
		log.Fatal("Error in creating NewConsumer.", err)
	}
	// Close the consumer
	defer c.Close()

	// Subscribe to topics
	err = c.SubscribeTopics(topics, nil)
	if err != nil {
		c.Close()
		log.Fatal("Error in SubscribeTopics.", err)
	}

	// Start consuming messages
	go consumeMessages(c)

	// Start capturing
	fmt.Println("Capturing. Point your browser to " + nodeport)

	// Start http server
	http.Handle("/", stream)
	log.Fatal(http.ListenAndServe(displayport, nil))
}

func getenvint(str string) int {
	i, err := strconv.Atoi(os.Getenv(str))
	if err != nil {
		log.Fatal(err)
	}
	return i
}

func consumeMessages(c *kafka.Consumer) {
	doc := &topicMsg{}

	// Consume messages
	for e := range c.Events() {
		switch ev := e.(type) {
		case kafka.AssignedPartitions:
			// log.Printf("%% %v\n", ev)
			c.Assign(ev.Partitions)
		case kafka.RevokedPartitions:
			// log.Printf("%% %v\n", ev)
			c.Unassign()
		case kafka.PartitionEOF:
			// log.Printf("%% Reached %v\n", ev)
		case kafka.Error:
			// Errors should generally be considered as informational, the client will try to automatically recover
			// log.Printf("%% Error: %v\n", ev)
		case *kafka.Message:

			//Read message into `topicMsg` struct
			err := json.Unmarshal(ev.Value, doc)
			if err != nil {
				// log.Println(err)
				continue
			}

			//Retrieve img
			// log.Printf("%% Message sent %v on %s\n", ev.Timestamp, ev.TopicPartition)
			img, err := gocv.NewMatFromBytes(doc.Rows, doc.Cols, doc.Type, doc.Mat)
			if err != nil {
				// log.Println("Frame:", err)
				continue
			}

			//Encode gocv mat to jpeg
			buf, err := gocv.IMEncode(gocv.JPEGFileExt, img)
			if err != nil {
				// log.Println("Error in IMEncode:", err)
				continue
			}

			stream.UpdateJPEG(buf)

		default:
			// log.Println("Ignored")
			continue
		}

		// //Record the current topic-partition assignments
		// tpSlice, err := c.Assignment()
		// if err != nil {
		// 	// log.Println(err)
		// 	continue
		// }

		// //Obtain the last message offset for all topic-partition
		// for index, tp := range tpSlice {
		// 	_, high, err := c.QueryWatermarkOffsets(*(tp.Topic), tp.Partition, 100)
		// 	if err != nil {
		// 		// log.Println(err)
		// 		continue
		// 	}
		// 	tpSlice[index].Offset = kafka.Offset(high)
		// }

		// //Consume the last message in topic-partition
		// c.Assign(tpSlice)
	}
}

type topicMsg struct {
	Mat      []byte       `json:"mat"`
	Channels int          `json:"channels"`
	Rows     int          `json:"rows"`
	Cols     int          `json:"cols"`
	Type     gocv.MatType `json:"type"`
}
