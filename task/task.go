package task

import (
	"bytes"
	"log"
	"net/http"

	"github.com/opensourceways/gotty/webtty"
	"github.com/robfig/cron"
)

func send() {
	if len(webtty.Log) != 0 {
		var logBak = make([]string, len(webtty.Log))
		copy(logBak, webtty.Log)
		webtty.Log = make([]string, 0)
		for _, v := range logBak {
			_, err := http.Post("http://localhost:8888/log.gotty", "application/json", bytes.NewReader([]byte(v)))
			if err != nil {
				log.Println(err)
				break
			}
		}
	}
}
func SendLog() {
	c := cron.New()
	err := c.AddFunc("0/30 * * * * ?", send)
	if err != nil {
		log.Fatalln(err)
	}
	c.Start()
}
