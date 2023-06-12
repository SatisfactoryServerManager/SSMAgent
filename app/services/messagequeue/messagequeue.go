package messagequeue

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/savemanager"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
)

var (
	_quit         = make(chan int)
	_messageQueue []MessageQueueItem
)

type MessageQueueItem struct {
	ID        string      `json:"_id"`
	Action    string      `json:"action"`
	Data      interface{} `json:"data"`
	Completed bool        `json:"completed"`
	Error     string      `json:"error"`
	Retries   int         `json:"retries"`
	Created   time.Time   `json:"created"`
}

type HttpRequestBody_MessageItem struct {
	Item MessageQueueItem `json:"item"`
}

func InitMessageQueue() {
	log.Println("Initialising Message Queue...")

	GetMessageQueue()

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				GetMessageQueue()
				ProcessAllMessageQueueItems()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()

	log.Println("Initialised Message Queue")
}

func ShutdownMessageQueue() error {
	log.Println("Shutting down Message Queue")

	_quit <- 0

	log.Println("Shutdown Message Queue")
	return nil
}

func GetMessageQueue() {

	err := api.SendGetRequest("/api/agent/messagequeue", &_messageQueue)
	if err != nil {
		log.Println(err.Error())
	}
}

func ProcessAllMessageQueueItems() {

	if len(_messageQueue) == 0 {
		return
	}

	for idx := range _messageQueue {
		messageQueueItem := &_messageQueue[idx]
		err := ProcessMessageQueueItem(messageQueueItem)

		if err != nil {
			messageQueueItem.Error = err.Error()
			messageQueueItem.Retries += 1
			continue
		}

		messageQueueItem.Completed = true
	}

	for idx := range _messageQueue {
		messageQueueItem := &_messageQueue[idx]

		itemBody := HttpRequestBody_MessageItem{Item: *messageQueueItem}

		var resData interface{}
		err := api.SendPostRequest("/api/agent/messagequeue", itemBody, &resData)

		if err != nil {
			fmt.Printf("Error sending message queue item update %s\r\n", messageQueueItem.ID)
		}

	}
}

func ProcessMessageQueueItem(messageItem *MessageQueueItem) error {

	fmt.Printf("Processing Message action %s\r\n", messageItem.Action)
	switch messageItem.Action {
	case "startsfserver":
		return sf.StartSFServer()
	case "stopsfserver":
		return sf.ShutdownSFServer()
	case "killsfserver":
		return sf.KillSFServer()
	case "installsfserver":
		return sf.InstallSFServer(true)
	case "updatesfserver":
		return sf.UpdateSFServer()
	case "downloadSave":
		var objmap map[string]json.RawMessage
		b, _ := json.Marshal(messageItem.Data)
		json.Unmarshal(b, &objmap)
		return savemanager.DownloadSaveFile(string(objmap["saveFile"]))
	default:
		return errors.New("unknown message queue action")
	}
}
