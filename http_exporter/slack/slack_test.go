package slack

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestPost(t *testing.T) {
	data, err := ioutil.ReadFile(`./webhookurl.txt`)
	if err != nil {
		log.Println("Cannot open file")
	}
	webhookurl := string(data)

	sc, err := NewSlack(webhookurl, "webmon", "test")
	if err != nil {
		t.Fatal(err)
	}

	if err := sc.Post("title", "<!here>", "test", "good"); err != nil {
		t.Fatal(err)
	}
}
