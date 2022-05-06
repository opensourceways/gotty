package task

import (
	"bytes"
	"log"
	"net/http"

	"github.com/opensourceways/gotty/server"
	"github.com/opensourceways/gotty/webtty"
)

var Option = new(server.Options)

func Send() {
	if len(webtty.Log) != 0 {
		var logBak = make([]string, len(webtty.Log))
		copy(logBak, webtty.Log)
		webtty.Log = make([]string, 0)
		for _, v := range logBak {
			_, err := http.Post(Option.SendUrl, "application/json", bytes.NewReader([]byte(v)))
			if err != nil {
				log.Println(err)
				break
			}
		}
	}
}
