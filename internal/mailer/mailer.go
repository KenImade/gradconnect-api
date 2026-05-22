package mailer

import (
	"bytes"
	"embed"
	"time"

	"github.com/wneessen/go-mail"

	ht "html/template"
	tt "text/template"
)

//go:embed templates
var templateFS embed.FS

type Mailer struct {
	client           *mail.Client
	sender           string
	configurationSet string
}

func New(host string, port int, username, password, sender string, tlsMandatory bool, configurationSet string) (*Mailer, error) {
	tlsPolicy := mail.TLSOpportunistic
	if tlsMandatory {
		tlsPolicy = mail.TLSMandatory
	}

	opts := []mail.Option{
		mail.WithPort(port),
		mail.WithTimeout(10 * time.Second),
		mail.WithTLSPolicy(tlsPolicy),
	}

	if username != "" || password != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthLogin),
			mail.WithUsername(username),
			mail.WithPassword(password),
		)
	} else {
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthNoAuth))
	}

	client, err := mail.NewClient(host, opts...)
	if err != nil {
		return nil, err
	}

	mailer := &Mailer{
		client:           client,
		sender:           sender,
		configurationSet: configurationSet,
	}

	return mailer, nil
}

func (m *Mailer) Send(recipient string, templateFile string, data any) error {
	textTmpl, err := tt.New(templateFile).ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}

	subject := new(bytes.Buffer)
	err = textTmpl.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err
	}

	plainBody := new(bytes.Buffer)
	err = textTmpl.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return err
	}

	htmlTmpl, err := ht.New(templateFile).ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}

	htmlBody := new(bytes.Buffer)
	err = htmlTmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err
	}

	msg := mail.NewMsg()

	err = msg.To(recipient)
	if err != nil {
		return err
	}

	err = msg.From(m.sender)
	if err != nil {
		return err
	}

	if m.configurationSet != "" {
		msg.SetGenHeader("X-SES-CONFIGURATION-SET", m.configurationSet)
	}

	msg.Subject(subject.String())
	msg.SetBodyString(mail.TypeTextPlain, plainBody.String())
	msg.AddAlternativeString(mail.TypeTextHTML, htmlBody.String())

	return m.client.DialAndSend(msg)
}
