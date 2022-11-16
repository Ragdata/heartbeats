package internal

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/gi8lino/heartbeats/internal/notifications"

	"github.com/mitchellh/mapstructure"
	"github.com/nikoksr/notify"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var HeartbeatsServer HeartbeatsConfig

const (
	EnvPrefix = "env:"
)

type Config struct {
	Path         string `mapstructure:"path"`
	PrintVersion bool   `mapstructure:"printVersion"`
	Debug        bool   `mapstructure:"debug"`
}

type Server struct {
	Hostname string `mapstructure:"hostname"`
	Port     int    `mapstructure:"port"`
}

type Heartbeat struct {
	Name          string        `mapstructure:"name"`
	Description   string        `mapstructure:"description"`
	Interval      time.Duration `mapstructure:"interval"`
	Grace         time.Duration `mapstructure:"grace"`
	LastPing      time.Time     `mapstructure:"lastPing"`
	Status        string        `mapstructure:"status"`
	Notifications []string      `mapstructure:"notifications"`
	IntervalTimer *Timer
	GraceTimer    *Timer
}

type Defaults struct {
	Subject string `mapstructure:"subject" default:"Heartbeat"`
	Message string `mapstructure:"message" default:"Heartbeat is missing"`
}

type Notifications struct {
	Defaults Defaults      `mapstructure:"defaults"`
	Services []interface{} `mapstructure:"services"`
}

type HeartbeatsConfig struct {
	Config        Config        `mapstructure:"config"`
	Server        Server        `mapstructure:"server"`
	Heartbeats    []Heartbeat   `mapstructure:"heartbeats"`
	Notifications Notifications `mapstructure:"notifications"`
	Version       string        `mapstructure:"version"`
}

// ReadConfigFile reads the notifications config file with and returns a Config struct
func ReadConfigFile(configPath string) error {
	// extract parent directory
	if strings.Contains(configPath, "/") { // set config file path
		configPath = configPath[strings.LastIndex(configPath, "/")+1:] // extract filename
	}

	fileType := configPath[strings.LastIndex(configPath, ".")+1:] // extract file type

	viper.SetConfigFile(configPath)
	viper.SetConfigType(fileType)

	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	if err := viper.Unmarshal(&HeartbeatsServer); err != nil {
		return err
	}

	if err := ProcessServiceSettings(); err != nil {
		return fmt.Errorf("error while processing notification services: %s", err)
	}

	if err := CheckSendDetails(); err != nil {
		return err
	}

	return nil
}

// ProcessNotificationSettings processes the list with notifications
func ProcessServiceSettings() error {
	for i, service := range HeartbeatsServer.Notifications.Services {
		var serviceType string

		// evaluate type of service
		// this is needed because the type of the service is not known at compile time
		switch service.(type) {
		case map[string]interface{}:
			s, ok := service.(map[string]interface{})["type"].(string)
			if !ok {
				return fmt.Errorf("type of service %s is not set", service.(map[string]interface{})["name"])
			}
			serviceType = s
		case notifications.SlackSettings:
			serviceType = "slack"
		case notifications.MailSettings:
			serviceType = "mail"
		default:
			return fmt.Errorf("invalid service type in notifications config file")
		}

		switch serviceType {
		case "slack":
			var result notifications.SlackSettings
			if err := mapstructure.Decode(service, &result); err != nil {
				return err
			}

			for name, value := range SubstituteFieldsWithEnv(EnvPrefix, result) {
				reflect.ValueOf(&result).Elem().FieldByName(name).Set(value)
			}

			svc, err := notifications.GenerateSlackService(result.OauthToken, result.Channels)
			if err != nil {
				return fmt.Errorf("error while generating slack service: %s", err)
			}
			result.Notifier = notify.New()
			result.Notifier.UseServices(svc)

			HeartbeatsServer.Notifications.Services[i] = result

			log.Debugf("Slack service «%s» is enabled: %t", result.Name, result.Enabled)

		case "mail":
			var result notifications.MailSettings
			if err := mapstructure.Decode(service, &result); err != nil {
				return err
			}

			for name, value := range SubstituteFieldsWithEnv(EnvPrefix, result) {
				reflect.ValueOf(&result).Elem().FieldByName(name).Set(value)
			}

			svc, err := notifications.GenerateMailService(result.SenderAddress, result.SmtpHostAddr, result.SmtpHostPort, result.SmtpAuthUser, result.SmtpAuthPassword, result.ReceiverAddresses)
			if err != nil {
				return fmt.Errorf("error while generating mail service: %s", err)
			}
			result.Notifier = notify.New()
			result.Notifier.UseServices(svc)

			HeartbeatsServer.Notifications.Services[i] = result

			log.Debugf("Mail service «%s» is enabled: %t", result.Name, result.Enabled)

		default:
			return fmt.Errorf("Unknown notification service type")
		}
	}
	return nil
}

// CheckSendDetails checks if the send details are set and parsing is possible
func CheckSendDetails() error {
	var heartbeat Heartbeat

	// check defaults
	if HeartbeatsServer.Notifications.Defaults.Subject == "" {
		return fmt.Errorf("default subject is not set")
	}
	if _, err := Substitute("default", HeartbeatsServer.Notifications.Defaults.Subject, heartbeat); err != nil {
		return err
	}

	if _, err := Substitute("default", HeartbeatsServer.Notifications.Defaults.Message, heartbeat); err != nil {
		return err
	}

	if HeartbeatsServer.Notifications.Defaults.Message == "" {
		return fmt.Errorf("default message is not set")
	}

	var name string
	var subject string
	var message string

	for _, notification := range HeartbeatsServer.Notifications.Services {
		switch notification.(type) {
		case notifications.SlackSettings:
			settings := notification.(notifications.SlackSettings)
			name = settings.Name
			subject = settings.Subject
			message = settings.Message
		case notifications.MailSettings:
			settings := notification.(notifications.MailSettings)
			name = settings.Name
			subject = settings.Subject
			message = settings.Message
		default:
			return fmt.Errorf("invalid service type in notifications config file")
		}

		if _, err := Substitute(name, subject, heartbeat); err != nil {
			return fmt.Errorf("Error in notification settings: %s", err)
		}
		if _, err := Substitute(name, message, heartbeat); err != nil {
			return fmt.Errorf("Error in notification settings: %s", err)
		}
	}
	return nil
}

// SubstituteFieldsWithEnv searches in env for the given key and replaces the value with the value from env
func SubstituteFieldsWithEnv(prefix string, a any) map[string]reflect.Value {
	result := make(map[string]reflect.Value)

	r := reflect.TypeOf(a)
	for i := 0; i < r.NumField(); i++ {
		field := r.Field(i)
		// get field value
		value := reflect.ValueOf(a).FieldByName(field.Name)
		if !strings.HasPrefix(value.String(), prefix) {
			continue
		}

		envValue := os.Getenv(value.String()[len(prefix):])
		if envValue != "" {
			reflectedValue := reflect.ValueOf(envValue)
			result[field.Name] = reflectedValue
		}
	}
	return result
}
