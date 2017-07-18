package mail

import (
	"crypto/tls"
	"fmt"
	"html/template"
	"io"
	"net/smtp"
	"strconv"
	"time"

	"github.com/moira-alert/notifier"
	gomail "gopkg.in/gomail.v2"
)

var tpl = template.Must(template.New("mail").Parse(`
<html>
	<head>
		<style type="text/css">
			table { border-collapse: collapse; }
			table th, table td { padding: 0.5em; }
			tr.OK { background-color: #33cc99; color: white; }
			tr.WARN { background-color: #cccc32; color: white; }
			tr.ERROR { background-color: #cc0032; color: white; }
			tr.NODATA { background-color: #d3d3d3; color: black; }
			tr.EXCEPTION { background-color: #e14f4f; color: white; }
			th, td { border: 1px solid black; }
		</style>
	</head>
	<body>
		<table>
			<thead>
				<tr>
					<th>Timestamp</th>
					<th>Target</th>
					<th>Value</th>
					<th>Warn</th>
					<th>Error</th>
					<th>From</th>
					<th>To</th>
					<th>Note</th>
				</tr>
			</thead>
			<tbody>
				{{range .Items}}
				<tr class="{{ .State }}">
					<td>{{ .Timestamp }}</td>
					<td>{{ .Metric }}</td>
					<td>{{ .Value }}</td>
					<td>{{ .WarnValue }}</td>
					<td>{{ .ErrorValue }}</td>
					<td>{{ .Oldstate }}</td>
					<td>{{ .State }}</td>
					<td>{{ .Message }}</td>
				</tr>
				{{end}}
			</tbody>
		</table>
		<p>Description: {{ .Description }}</p>
		<p><a href="{{ .Link }}">{{ .Link }}</a></p>
		{{if .Throttled}}
		<p>Please, <b>fix your system or tune this trigger</b> to generate less events.</p>
		{{end}}
	</body>
</html>
`))

var log notifier.Logger

type templateRow struct {
	Metric     string
	Timestamp  string
	Oldstate   string
	State      string
	Value      string
	WarnValue  string
	ErrorValue string
	Message    string
}

// Sender implements moira sender interface via pushover
type Sender struct {
	From        string
	SMTPhost    string
	SMTPport    int64
	FrontURI    string
	InsecureTLS bool
	Password    string
	Username    string
	SSL         bool
}

// Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, logger notifier.Logger) error {
	sender.SetLogger(logger)
	sender.From = senderSettings["mail_from"]
	sender.SMTPhost = senderSettings["smtp_host"]
	sender.SMTPport, _ = strconv.ParseInt(senderSettings["smtp_port"], 10, 64)
	sender.InsecureTLS, _ = strconv.ParseBool(senderSettings["insecure_tls"])
	sender.FrontURI = senderSettings["front_uri"]
	sender.Password = senderSettings["smtp_pass"]
	sender.Username = senderSettings["smtp_user"]
	if sender.Username == "" {
		sender.Username = sender.From
	}
	sender.SSL, _ = strconv.ParseBool(senderSettings["ssl"])

	if sender.From == "" {
		return fmt.Errorf("mail_from can't be empty")
	}
	t, err := smtp.Dial(fmt.Sprintf("%s:%d", sender.SMTPhost, sender.SMTPport))
	if err != nil {
		return err
	}
	defer t.Close()
	if sender.Password != "" {
		err = t.StartTLS(&tls.Config{
			InsecureSkipVerify: sender.InsecureTLS,
			ServerName:         sender.SMTPhost,
		})
		if err != nil {
			return err
		}
		err = t.Auth(smtp.PlainAuth("", sender.Username, sender.Password, sender.SMTPhost))
		if err != nil {
			return err
		}
	}
	return nil
}

// SetLogger for test purposes
func (sender *Sender) SetLogger(logger notifier.Logger) {
	log = logger
}

// MakeMessage prepare message to send
func (sender *Sender) MakeMessage(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) *gomail.Message {
	state := events.GetSubjectState()
	tags := trigger.GetTags()

	subject := fmt.Sprintf("%s %s %s (%d)", state, trigger.Name, tags, len(events))

	templateData := struct {
		Link        string
		Description string
		Throttled   bool
		Items       []*templateRow
	}{
		Link:        fmt.Sprintf("%s/#/events/%s", sender.FrontURI, events[0].TriggerID),
		Description: trigger.Desc,
		Throttled:   throttled,
		Items:       make([]*templateRow, 0, len(events)),
	}

	for _, event := range events {
		templateData.Items = append(templateData.Items, &templateRow{
			Metric:     event.Metric,
			Timestamp:  time.Unix(event.Timestamp, 0).Format("15:04 02.01.2006"),
			Oldstate:   event.OldState,
			State:      event.State,
			Value:      strconv.FormatFloat(event.Value, 'f', -1, 64),
			WarnValue:  strconv.FormatFloat(trigger.WarnValue, 'f', -1, 64),
			ErrorValue: strconv.FormatFloat(trigger.ErrorValue, 'f', -1, 64),
			Message:    event.Message,
		})
	}

	m := gomail.NewMessage()
	m.SetHeader("From", sender.From)
	m.SetHeader("To", contact.Value)
	m.SetHeader("Subject", subject)
	m.AddAlternativeWriter("text/html", func(w io.Writer) error {
		return tpl.Execute(w, templateData)
	})

	return m
}

//SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {

	m := sender.MakeMessage(events, contact, trigger, throttled)

	d := gomail.Dialer{
		Host: sender.SMTPhost,
		Port: int(sender.SMTPport),
		TLSConfig: &tls.Config{
			InsecureSkipVerify: sender.InsecureTLS,
			ServerName:         sender.SMTPhost,
		},
		SSL: sender.SSL,
	}

	if sender.Password != "" {
		d.Auth = smtp.PlainAuth("", sender.Username, sender.Password, sender.SMTPhost)
	}

	if err := d.DialAndSend(m); err != nil {
		return err
	}
	return nil
}
