package email

import (
	"fmt"
	gomail "gopkg.in/mail.v2"

	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/logging"

	"github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, config *conf.ConfigSection) {
	fromAddress, err := config.GetString("smtp_from")
	if err != nil {
		logger.Fatal(fmt.Sprintf("email: smtp_from not set in config - %v", err))
		return
	}

	toAddress, err := config.GetString("smtp_to")
	if err != nil {
		logger.Fatal(fmt.Sprintf("email: smtp_to not set in config - %v", err))
		return
	}

	smtpUsername, err := config.GetString("smtp_username")
	if err != nil {
		logger.Fatal(fmt.Sprintf("email: smtp_username not set in config - %v", err))
		return
	}

	smtpPassword, err := config.GetString("smtp_password")
	if err != nil {
		logger.Fatal(fmt.Sprintf("email: smtp_password not set in config - %v", err))
		return
	}

	smtpHost, err := config.GetString("smtp_host")
	if err != nil {
		logger.Fatal(fmt.Sprintf("email: smtp_host not set in config - %v", err))
		return
	}

	smtpPort, err := config.GetInt("smtp_port")
	if err != nil {
		logger.Fatal(fmt.Sprintf("email: smtp_port not set in config - %v", err))
		return
	}

	subEmail, _ := bus.Subscribe("email:send")
	defer subEmail.Close()

	for event := range subEmail.Ch {
		if event.Type != "email" {
			continue
		}

		m := gomail.NewMessage()
		m.SetHeader("From", fromAddress)
		m.SetHeader("To", toAddress)
		m.SetHeader("Subject", event.Email.Subject)
		m.SetBody("text/plain", event.Email.Body)

		d := gomail.NewDialer(smtpHost, smtpPort, smtpUsername, smtpPassword)

		if err := d.DialAndSend(m); err != nil {
			logger.Error(fmt.Sprintf("email: failed to send (%v)", err))
			continue
		}
	}
}
