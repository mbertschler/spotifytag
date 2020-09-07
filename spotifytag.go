package spotifytag

import (
	"bytes"
	"log"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2"
	"github.com/pkg/errors"
	"github.com/zmb3/spotify"
)

var reverseTags = map[string]string{}

func init() {
	for k, v := range id3v2.V23CommonIDs {
		reverseTags[v] = k
	}
	for k, v := range id3v2.V24CommonIDs {
		reverseTags[v] = k
	}
}

func Analyze(dir string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "*.mp3"))
	if err != nil {
		return errors.Wrap(err, "filepath.Glob")
	}
	if len(matches) == 0 {
		return errors.New("no files found in " + dir)
	}

	for _, file := range matches {
		tag, err := analyzeFile(file)
		if err != nil {
			return err
		}

		name := filepath.Base(file)
		name = strings.TrimSuffix(name, ".mp3")
		track, err := fetchFromAPI(name, tag)
		if err != nil {
			return err
		}

		tag.DeleteFrames("TPE1")
		tag.DeleteFrames("TPE2")
		tag.DeleteFrames("APIC")
		tag.SetTitle(track.Name)
		tag.SetArtist(artistNames(track.Artists))
		tag.SetAlbum(track.Album.Name)
		tag.SetYear(track.Album.ReleaseDate)

		var largestImg spotify.Image
		for _, img := range track.Album.Images {
			if img.Width > largestImg.Width {
				largestImg = img
			}
		}
		buf := &bytes.Buffer{}
		err = largestImg.Download(buf)
		if err != nil {
			return err
		}

		pic := id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    "image/jpeg",
			PictureType: id3v2.PTFrontCover,
			Description: "Cover",
			Picture:     buf.Bytes(),
		}
		tag.AddAttachedPicture(pic)

		err = tag.Save()
		if err != nil {
			return err
		}

		err = tag.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func analyzeFile(path string) (*id3v2.Tag, error) {
	log.Println("Path", path)
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		log.Fatal("Error while opening mp3 file: ", err)
	}

	// log.Println("version", tag.Version())

	return tag, nil

	// Read tags.
	// log.Println(tag.Artist())
	// log.Println(tag.Title())
	frames := tag.AllFrames()

	for name, framers := range frames {
		log.Println("Frame", name, reverseTags[name])
		for i, frame := range framers {
			var txt string
			switch v := frame.(type) {
			case id3v2.TextFrame:
				txt = v.Text
			}
			log.Printf("    %d %T %q", i, frame, txt)
		}
	}

	return tag, nil

	// // Set tags.
	// tag.SetArtist("Aphex Twin")
	// tag.SetTitle("Xtal")

	// comment := id3v2.CommentFrame{
	// 	Encoding:    id3v2.EncodingUTF8,
	// 	Language:    "eng",
	// 	Description: "My opinion",
	// 	Text:        "I like this song!",
	// }
	// tag.AddCommentFrame(comment)

	// // Write tag to file.mp3.
	// if err = tag.Save(); err != nil {
	// 	log.Fatal("Error while saving a tag: ", err)
	// }
}
