package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"crypto/tls"
)

// used for generating salt
var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

type SubsonicConnection struct {
	Username              string
	Password              string
	Host                  string
	AcceptInvalidSslCert  bool
}

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func authToken(password string) (string, string) {
	salt := randSeq(8)
	token := fmt.Sprintf("%x", md5.Sum([]byte(password+salt)))

	return token, salt
}

func defaultQuery(connection *SubsonicConnection) url.Values {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: connection.AcceptInvalidSslCert}
	token, salt := authToken(connection.Password)
	query := url.Values{}
	query.Set("u", connection.Username)
	query.Set("t", token)
	query.Set("s", salt)
	query.Set("v", "1.15.1")
	query.Set("c", "stmp")
	query.Set("f", "json")

	return query
}

// response structs
type SubsonicError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type SubsonicSong struct {
	Id          string `json:"id"`
	Artist      string `json:"artist"`
	Title       string `json:"title"`
	Duration    int    `json:"duration"`
	Track       int    `json:"track"`
	DiskNumber  int    `json:"diskNumber"`
	Path        string `json:"path"`
}

type SubsonicSongAlbum struct {
	Songs       []SubsonicSong  `json:"song"`
        ArtistId    string          `json:"artistId"`
}

type SubsonicSongResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Album     SubsonicSongAlbum    `json:"album"`
	Error     SubsonicError     `json:"error"`
}

type Song struct {
	Response SubsonicSongResponse `json:"subsonic-response"`
}

type SubsonicAlbum struct {
	Id          string `json:"id"`
	Title       string `json:"name"`
	Duration    int    `json:"duration"`
}

type SubsonicAlbumArtist struct {
	Albums       []SubsonicAlbum `json:"album"`
}

type SubsonicAlbumResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Artist    SubsonicAlbumArtist   `json:"artist"`
	Error     SubsonicError     `json:"error"`
}

type Album struct {
	Response SubsonicAlbumResponse `json:"subsonic-response"`
}

type SubsonicRandomSong struct {
	Songs       []SubsonicSong  `json:"song"`
}

type SubsonicRandomSongsResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	RandomSongs    SubsonicRandomSong   `json:"randomSongs"`
	Error     SubsonicError     `json:"error"`
}

type RandomSongs struct {
	Response SubsonicRandomSongsResponse `json:"subsonic-response"`
}

type SubsonicArtist struct {
	Id         string
	Name       string
	AlbumCount int
}

type SubsonicIndex struct {
	Name    string           `json:"name"`
	Artists []SubsonicArtist `json:"artist"`
}

type SubsonicIndexes struct {
	Index []SubsonicIndex
}

type SubsonicArtistsResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Indexes   SubsonicIndexes   `json:"artists"`
	Error     SubsonicError     `json:"error"`
}

type Artists struct {
	Response SubsonicArtistsResponse `json:"subsonic-response"`
}

type SubsonicPingResponse struct {
  Status    string            `json:"status"`
  Version   string            `json:"version"`
}

type Ping struct {
	Response SubsonicPingResponse `json:"subsonic-response"`
}

// requests
func (connection *SubsonicConnection) GetServerInfo() (*SubsonicPingResponse, error) {
	query := defaultQuery(connection)
	requestUrl := connection.Host + "/rest/ping?" + query.Encode()
	res, err := http.Get(requestUrl)

	if err != nil {
		return nil, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	responseBody, readErr := ioutil.ReadAll(res.Body)

	if readErr != nil {
		return nil, err
	}

	var decodedBody Ping
	err = json.Unmarshal(responseBody, &decodedBody)

	if err != nil {
		return nil, err
	}

	return &decodedBody.Response, nil
}

func (connection *SubsonicConnection) GetRandomSongs(count int) (*SubsonicRandomSongsResponse, error) {
	query := defaultQuery(connection)
	requestUrl := fmt.Sprintf("%s/rest/getRandomSongs?%s&size=%d", connection.Host, query.Encode(), count)
	res, err := http.Get(requestUrl)

	if err != nil {
		return nil, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	responseBody, readErr := ioutil.ReadAll(res.Body)

	if readErr != nil {
		return nil, err
	}

	var decodedBody RandomSongs
	err = json.Unmarshal(responseBody, &decodedBody)

	if err != nil {
		return nil, err
	}

	return &decodedBody.Response, nil
}

func (connection *SubsonicConnection) GetArtists() (*SubsonicArtistsResponse, error) {
	query := defaultQuery(connection)
	requestUrl := connection.Host + "/rest/getArtists?" + query.Encode()
	res, err := http.Get(requestUrl)

	if err != nil {
		return nil, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	responseBody, readErr := ioutil.ReadAll(res.Body)

	if readErr != nil {
		return nil, err
	}

	var decodedBody Artists
	err = json.Unmarshal(responseBody, &decodedBody)

	if err != nil {
		return nil, err
	}

	return &decodedBody.Response, nil
}

func (connection *SubsonicConnection) GetArtist(id string) (*SubsonicAlbumResponse, error) {
	query := defaultQuery(connection)
	query.Set("id", id)
	requestUrl := connection.Host + "/rest/getArtist?" + query.Encode()
	res, err := http.Get(requestUrl)

	if err != nil {
		return nil, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	responseBody, readErr := ioutil.ReadAll(res.Body)

	if readErr != nil {
		return nil, err
	}

	var decodedBody Album
	err = json.Unmarshal(responseBody, &decodedBody)

	if err != nil {
		return nil, err
	}

	return &decodedBody.Response, nil
}

func (connection *SubsonicConnection) GetAlbum(id string) (*SubsonicSongResponse, error) {
	query := defaultQuery(connection)
	query.Set("id", id)
	requestUrl := connection.Host + "/rest/getAlbum?" + query.Encode()
	res, err := http.Get(requestUrl)

	if err != nil {
		return nil, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	responseBody, readErr := ioutil.ReadAll(res.Body)

	if readErr != nil {
		return nil, err
	}

	var decodedBody Song
	err = json.Unmarshal(responseBody, &decodedBody)

	if err != nil {
		return nil, err
	}

	return &decodedBody.Response, nil
}

// note that this function does not make a request, it just formats the play url
// to pass to mpv
func (connection *SubsonicConnection) GetPlayUrl(song *SubsonicSong) string {
	query := defaultQuery(connection)
	query.Set("id", song.Id)
	return connection.Host + "/rest/stream?" + query.Encode()
}

