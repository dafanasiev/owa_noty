package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/0xAX/notificator"
	"github.com/kardianos/osext"

	"github.com/dafanasiev/owa_noty/EWS2010sp1"
)

var BuildTime = "N/A"
var BuildGitHash = "N/A"

var notify *notificator.Notificator

type Configuration struct {
	Endpoint string `json:"endpoint"`
	Username string `json:"username"`
	Password string `json:"password"`

	Title string `json:"title"`
	Text  string `json:"text"`
}

func main() {
	log.Printf("OWA_Noty. Build at: %s; version: %s", BuildTime, BuildGitHash)
	log.Println("----------------------------------------")

	const cfgFileName = "config.json"
	const newMailIcon = "newmail.png"

	selfDir, err := os.Getwd()
	cfgFileNameFull := filepath.Join(selfDir, cfgFileName)
	if _, err := os.Stat(cfgFileNameFull); os.IsNotExist(err) {
		selfDir, _ = osext.ExecutableFolder()
		cfgFileNameFull = filepath.Join(selfDir, cfgFileName)
		if _, err := os.Stat(cfgFileNameFull); os.IsNotExist(err) {
			log.Panicf("Cant find %s file", cfgFileName)
		}
	}

	cfgFile, err := os.Open(cfgFileNameFull)
	if err != nil {
		log.Panicf("Unable to open %s", cfgFileName)
	}

	decoder := json.NewDecoder(cfgFile)
	cfg := Configuration{}
	err = decoder.Decode(&cfg)

	if err != nil {
		log.Panicf("Unable to parse %s", cfgFileName)
	}

	newMailIconFull := filepath.Join(selfDir, newMailIcon)

	notify = notificator.New(notificator.Options{
		DefaultIcon: newMailIconFull,
		AppName:     "OWA Noty",
	})

	c := EWS2010sp1.NewClient(cfg.Endpoint)
	s := c.SubscribeNewMessages(context.Background(), cfg.Username, cfg.Password, func(ctx context.Context, err error, eArgs *EWS2010sp1.NewMessageEventArgs) {
		if err != nil {
			log.Printf("%v", err)
		} else {
			title := cfg.Title
			if eArgs.From != "" {
				title += " " + eArgs.From
			}
			if eArgs.FromEmail != "" {
				title += " <" + eArgs.FromEmail + ">"
			}

			if title == "" {
				title = cfg.Title
			}

			text := cfg.Text
			if eArgs.Subject != "" {
				text = eArgs.Subject
			}

			notify.Push(title, text, newMailIconFull, notificator.UR_NORMAL)
		}
	})
	defer s.Dispose()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)

	<-signalCh
}
