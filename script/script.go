package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/moira-alert/notifier"
)

var log notifier.Logger

// Sender implements moira sender interface via script execution
type Sender struct {
	Exec string
}

type scriptNotification struct {
	Events    []notifier.EventData `json:"events"`
	Trigger   notifier.TriggerData `json:"trigger"`
	Contact   notifier.ContactData `json:"contact"`
	Throttled bool                 `json:"throttled"`
	Timestamp int64                `json:"timestamp"`
}

//Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, logger notifier.Logger) error {
	if senderSettings["name"] == "" {
		return fmt.Errorf("Required name for sender type script")
	}
	args := strings.Split(senderSettings["exec"], " ")
	scriptFile := args[0]
	infoFile, err := os.Stat(scriptFile)
	if err != nil {
		return fmt.Errorf("File %s not found", scriptFile)
	}
	if !infoFile.Mode().IsRegular() {
		return fmt.Errorf("%s not file", scriptFile)
	}
	sender.Exec = senderSettings["exec"]
	log = logger
	return nil
}

//SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {

	execString := strings.Replace(sender.Exec, "${trigger_name}", trigger.Name, -1)
	execString = strings.Replace(execString, "${contact_value}", contact.Value, -1)

	args := strings.Split(execString, " ")
	scriptFile := args[0]
	infoFile, err := os.Stat(scriptFile)
	if err != nil {
		return fmt.Errorf("File %s not found", scriptFile)
	}
	if !infoFile.Mode().IsRegular() {
		return fmt.Errorf("%s not file", scriptFile)
	}

	scriptMessage := &scriptNotification{
		Events:    events,
		Trigger:   trigger,
		Contact:   contact,
		Throttled: throttled,
	}
	scriptJSON, err := json.MarshalIndent(scriptMessage, "", "\t")
	if err != nil {
		return fmt.Errorf("Failed marshal json")
	}

	c := exec.Command(scriptFile, args[1:]...)
	var scriptOutput bytes.Buffer
	c.Stdin = bytes.NewReader(scriptJSON)
	c.Stdout = &scriptOutput
	log.Debugf("Executing script: %s", scriptFile)
	err = c.Run()
	log.Debugf("Finished executing: %s", scriptFile)

	if err != nil {
		return fmt.Errorf("Failed exec [%s] Error [%s] Output: [%s]", sender.Exec, err.Error(), scriptOutput.String())
	}
	return nil
}
