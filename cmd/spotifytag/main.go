package main

import (
	"log"

	"github.com/mbertschler/spotifytag"
)

var dir = "/Users/mbertschler/Music/Spotify"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	err := spotifytag.Analyze(dir)
	if err != nil {
		log.Fatal(err)
	}
}
