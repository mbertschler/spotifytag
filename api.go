package spotifytag

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bogem/id3v2"
	"github.com/pkg/errors"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

// This example demonstrates how to authenticate with Spotify using the authorization code flow.
// In order to run this example yourself, you'll need to:
//
//  1. Register an application at: https://developer.spotify.com/my-applications/
//       - Use "http://localhost:4002" as the redirect URI
//  2. Set the SPOTIFY_ID environment variable to the client ID you got in step 1.
//  3. Set the SPOTIFY_SECRET environment variable to the client secret from step 1.

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const redirectURI = "http://localhost:4002/callback"

var (
	auth = spotify.NewAuthenticator(redirectURI,
		spotify.ScopeUserReadPrivate, spotify.ScopePlaylistReadPrivate)
	tokenChan = make(chan *oauth2.Token)
	state     = strconv.FormatInt(rand.Int63(), 32)

	currentSpotifyClient *spotify.Client

	verboseAPI = false
)

func spotifyClient() (*spotify.Client, error) {
	if currentSpotifyClient != nil {
		return currentSpotifyClient, nil
	}

	tokenPath, err := userTokenPath()
	if err != nil {
		return nil, err
	}
	buf, err := ioutil.ReadFile(tokenPath)
	if err == nil {
		tok := oauth2.Token{}
		err = json.Unmarshal(buf, &tok)
		if err != nil {
			return nil, err
		}
		client := auth.NewClient(&tok)
		currentSpotifyClient = &client
		return currentSpotifyClient, nil
	}

	// first start an HTTP server
	http.HandleFunc("/callback", completeAuth)
	go http.ListenAndServe(":4002", nil)

	url := auth.AuthURL(state)
	err = exec.Command("open", url).Run()
	if err != nil {
		return nil, err
	}

	// wait for auth to complete
	tok := <-tokenChan

	// use the token to get an authenticated client
	client := auth.NewClient(tok)
	currentSpotifyClient = &client
	return currentSpotifyClient, nil
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Println("can't get token:", err)
		return
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}

	tokenPath, err := userTokenPath()
	if err != nil {
		http.Error(w, "Couldn't get user", http.StatusInternalServerError)
		log.Println("can't get user:", err)
		return
	}

	buf, err := json.Marshal(tok)
	if err != nil {
		http.Error(w, "Couldn't make JSON", http.StatusInternalServerError)
		log.Println("can't make JSON:", err)
		return
	}

	err = os.MkdirAll(filepath.Dir(tokenPath), 0755)
	if err != nil {
		http.Error(w, "Couldn't create dir", http.StatusInternalServerError)
		log.Println("can't create dir:", err)
		return
	}

	err = ioutil.WriteFile(tokenPath, buf, 0644)
	if err != nil {
		http.Error(w, "Couldn't write token", http.StatusInternalServerError)
		log.Println("can't write token:", err)
		return
	}

	tokenChan <- tok
	fmt.Fprintf(w, "Login Completed!")
}

func fetchFromAPI(filename string, tag *id3v2.Tag) (*spotify.FullTrack, error) {
	client, err := spotifyClient()
	if err != nil {
		return nil, err
	}

	// use the client to make calls that require authorization
	// user, err := client.CurrentUser()
	// if err != nil {
	// 	return err
	// }
	// log.Println("You are logged in as:", user.ID)

	parts := strings.Fields(filename)
	search := ""

	for _, part := range parts {
		if len(part) > 1 {
			search += part + " "
		}
	}

	result, err := client.Search(search, spotify.SearchTypeTrack)
	if err != nil {
		return nil, err
	}

	track, err := chooseTrack(filename, result.Tracks)
	found := ""
	if err != nil {
		found = fmt.Sprintf("nothing - %v", err)
	} else {
		found = fmt.Sprint(artistNames(track.Artists), " - ", track.Name, " ", track.TimeDuration())
	}
	log.Printf("Seaching for %q, found %s", search, found)
	return track, nil
}

type scoredTrack struct {
	Score  int
	Length int
	spotify.FullTrack
}

func chooseTrack(filename string, tracks *spotify.FullTrackPage) (*spotify.FullTrack, error) {
	if len(tracks.Tracks) == 0 {
		return nil, errors.New("search results empty")
	}

	checkWords := []string{}
	for _, part := range strings.Fields(filename) {
		if len(part) > 1 {
			checkWords = append(checkWords, strings.ToLower(part))
		}
	}

	scored := []scoredTrack{}
	for i, t := range tracks.Tracks {

		s := scoredTrack{
			FullTrack: t,
		}
		artistsTrackString := strings.ToLower(artistNames(t.Artists) + " " + t.Name)
		s.Length = len(artistsTrackString)
		for _, w := range checkWords {
			if strings.Index(artistsTrackString, w) >= 0 {
				s.Score++
			}
		}
		if verboseAPI {
			log.Println("    ", i, artistNames(t.Artists), "-", t.Name,
				t.TimeDuration(), "score", s.Score, "len", s.Length)
		}
		scored = append(scored, s)
	}
	var highestScored scoredTrack
	for _, s := range scored {
		if s.Score == highestScored.Score {
			if s.Length < highestScored.Score {
				highestScored = s
			}
		} else if s.Score > highestScored.Score {
			highestScored = s
		}
	}
	return &highestScored.FullTrack, nil
}

func artistNames(in []spotify.SimpleArtist) string {
	names := []string{}
	for _, a := range in {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

func getPlaylist() {
	// playlist, err := client.GetPlaylist("5VpNRcTwzRGjcAeeoELypz")
	// if err != nil {
	// 	log.Println(err)
	// 	log.Fatalf("%#v", err.(*spotify.Error))
	// }

	// playlist.Tracks.
	// 	log.Println("Loaded playlist", playlist.Name)
	// for i, track := range playlist.Tracks.Tracks {
	// 	if i == 10 {
	// 		return
	// 	}
	// 	track.Track.re
	// 	track.Track.Album.Images
	// 	log.Println(i, track.Track.Artists, track.Track.Name, track.Track.Album.Name, track.Track.TimeDuration())
	// }
	// playlist.Tra
}

func userTokenPath() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}

	return filepath.Join(user.HomeDir, ".config", "spotifytag", "token.json"), nil
}
