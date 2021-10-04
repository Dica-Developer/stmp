package main

import (
  "fmt"
  "math"

  "github.com/gdamore/tcell/v2"
  "github.com/rivo/tview"
  "github.com/yourok/go-mpv/mpv"
)

type Entity struct {
  song    *SubsonicSong
  album   *SubsonicAlbum
}

/// struct contains all the updatable elements of the Ui
type Ui struct {
  app              *tview.Application
  entityList       *tview.List
  queueList        *tview.List
  startStopStatus  *tview.TextView
  playerStatus     *tview.TextView
  artistIdList     []string
  connection       *SubsonicConnection
  player           *Player
  currentEntities  []*Entity
}

func handleArtistSelected(artistId string, ui *Ui) {
  // TODO handle error here
  response, _ := ui.connection.GetArtist(artistId)

  ui.entityList.Clear()
  ui.currentEntities = []*Entity{}

  for _, album := range response.Artist.Albums {
    albumCopy := album
    ui.currentEntities = append(ui.currentEntities, &Entity{song: nil, album: &albumCopy})
    var title string
    var handler func()
    title = tview.Escape("[" + album.Title + "]")
    handler = makeAlbumHandler(album.Id, ui)

    ui.entityList.AddItem(title, "", 0, handler)
  }
}

func handleAlbumSelected(albumId string, ui *Ui) {
  response, _ := ui.connection.GetAlbum(albumId)

  ui.entityList.Clear()
  ui.entityList.AddItem(tview.Escape("[..]"), "", 0, makeArtistHandler(response.Album.ArtistId, ui))
  ui.currentEntities = []*Entity{}
  ui.currentEntities = append(ui.currentEntities, &Entity{nil, nil})

  for _, song := range response.Album.Songs {
    songCopy := song
    ui.currentEntities = append(ui.currentEntities, &Entity{song: &songCopy, album: nil})
    var title string
    var handler func()
    title = song.Title
    handler = makeSongHandler(song, title, song.Artist, song.Duration, ui.player, ui.queueList, ui)
    ui.entityList.AddItem(title, "", 0, handler)
  }
}

func handleDeleteFromQueue(ui *Ui) {
  currentIndex := ui.queueList.GetCurrentItem()
  queue := ui.player.Queue

  if currentIndex < 0 || len(ui.player.Queue) < currentIndex {
    return
  }

  // if the deleted item was the first one, and the player is loaded
  // remove the track. Removing the track auto starts the next one
  if currentIndex == 0 && ui.player.IsSongLoaded() {
    ui.player.Stop()
    return
  }

  // remove the item from the queue
  if len(ui.player.Queue) > 1 {
    ui.player.Queue = append(queue[:currentIndex], queue[currentIndex+1:]...)
  } else {
    ui.player.Queue = nil
  }

  updateQueueList(ui.player, ui.queueList)
}

func handleAddSongAlbumToQueue(ui *Ui) {
  currentIndex := ui.entityList.GetCurrentItem()

  if currentIndex < 0 || len(ui.currentEntities) < currentIndex {
    return
  }

  entity := ui.currentEntities[currentIndex]
  if entity.song == nil && entity.album == nil {
    return
  }

  if entity.album != nil {
    addAlbumToQueue(entity.album, ui)
  } else if entity.song != nil {
    addSongToQueue(entity.song, ui)
  }
  updateQueueList(ui.player, ui.queueList)
}

func addAlbumToQueue(album *SubsonicAlbum, ui *Ui) {
  response, _ := ui.connection.GetAlbum(album.Id)

  for _, e := range response.Album.Songs {
    addSongToQueue(&e, ui)
  }
}

func addSongToQueue(song *SubsonicSong, ui *Ui) {
  uri := ui.connection.GetPlayUrl(song)
  queueItem := QueueItem{
    uri,
    song.Title,
    song.Artist,
    song.Duration,
  }
  ui.player.Queue = append(ui.player.Queue, queueItem)
}

func makeSongHandler(song SubsonicSong, title string, artist string, duration int, player *Player,
queueList *tview.List, ui *Ui) func() {
  return func() {
    var uri string = ui.connection.GetPlayUrl(&song)
    player.Play(uri, title, artist, duration)
    updateQueueList(player, queueList)
  }
}

func makeArtistHandler(artistId string, ui *Ui) func() {
  return func() {
    handleArtistSelected(artistId, ui)
  }
}

func makeAlbumHandler(albumId string, ui *Ui) func() {
  return func() {
    handleAlbumSelected(albumId, ui)
  }
}

func InitGui(indexes *[]SubsonicIndex, connection *SubsonicConnection) *Ui {
  app := tview.NewApplication()
  entityList := tview.NewList().ShowSecondaryText(false).
  SetSelectedFocusOnly(true)
  queueList := tview.NewList().ShowSecondaryText(false)
  startStopStatus := tview.NewTextView().SetText("[::b]stmp: [red]stopped").
  SetTextAlign(tview.AlignLeft).
  SetDynamicColors(true)
  playerStatus := tview.NewTextView().SetText("[::b][100%][0:00/0:00]").
  SetTextAlign(tview.AlignRight).
  SetDynamicColors(true)
  player, err := InitPlayer()
  var artistIdList []string
  var currentEntities []*Entity

  ui := Ui{
    app,
    entityList,
    queueList,
    startStopStatus,
    playerStatus,
    artistIdList,
    connection,
    player,
    currentEntities,
  }

  if err != nil {
    app.Stop()
    fmt.Println("Unable to initialize mpv. Is mpv installed?")
  }

  //title row flex
  titleFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
  AddItem(startStopStatus, 0, 1, false).
  AddItem(playerStatus, 0, 1, false)

  // artist list, used to map the index of
  artistList := tview.NewList().ShowSecondaryText(false)
  for _, index := range *indexes {
    for _, artist := range index.Artists {
      artistList.AddItem(artist.Name, "", 0, nil)
      artistIdList = append(artistIdList, artist.Id)
    }
  }

  artistFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
  AddItem(artistList, 0, 1, true).
  AddItem(entityList, 0, 1, false)

  browserFlex := tview.NewFlex().SetDirection(tview.FlexRow).
  AddItem(titleFlex, 1, 0, false).
  AddItem(artistFlex, 0, 1, true)

  // going right from the artist list should focus the album/song list
  artistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    if event.Key() == tcell.KeyRight {
      app.SetFocus(entityList)
      return nil
    }
    return event
  })

  entityList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    if event.Key() == tcell.KeyLeft {
      app.SetFocus(artistList)
      return nil
    }
    if event.Rune() == 'a' {
      handleAddSongAlbumToQueue(&ui)
      return nil
    }

    return event
  })

  queueList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    if event.Key() == tcell.KeyDelete || event.Rune() == 'd' {
      handleDeleteFromQueue(&ui)
      return nil
    }

    return event
  })

  artistList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
    handleArtistSelected(artistIdList[index], &ui)
  })

  queueFlex := tview.NewFlex().SetDirection(tview.FlexRow).
  AddItem(titleFlex, 1, 0, false).
  AddItem(queueList, 0, 1, true)

  // handle
  go handleMpvEvents(&ui)

  pages := tview.NewPages().
  AddPage("browser", browserFlex, true, true).
  AddPage("queue", queueFlex, true, false)

  pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    if event.Rune() == '1' {
      pages.SwitchToPage("browser")
      return nil
    }
    if event.Rune() == '2' {
      pages.SwitchToPage("queue")
      return nil
    }
    if event.Rune() == 'q' {
      player.EventChannel <- nil
      player.Instance.TerminateDestroy()
      app.Stop()
    }
    if event.Rune() == 'D' {
      player.Queue = nil
      player.Stop()
      updateQueueList(player, queueList)
    }
    if event.Rune() == 'r' {
      addRandomSongsToQueue(&ui)
    }

    if event.Rune() == 'p' {
      status := player.Pause()
      if status == PlayerStopped {
        startStopStatus.SetText("[::b]stmp: [red]stopped")
      } else if status == PlayerPlaying {
        startStopStatus.SetText("[::b]stmp: [green]playing " + player.Queue[0].Title)
      } else if status == PlayerPaused {
        startStopStatus.SetText("[::b]stmp: [yellow]paused")
      }
      return nil
    }

    if event.Rune() == '-' {
      player.AdjustVolume(-5)
      return nil
    }

    if event.Rune() == '=' {
      player.AdjustVolume(5)
      return nil
    }

    return event
  })

  if err := app.SetRoot(pages, true).SetFocus(pages).EnableMouse(true).Run(); err != nil {
    panic(err)
  }

  return &ui
}

func updateQueueList(player *Player, queueList *tview.List) {
  queueList.Clear()
  for _, queueItem := range player.Queue {
    min, sec := iSecondsToMinAndSec(queueItem.Duration)
    queueList.AddItem(fmt.Sprintf("%s - %s - %02d:%02d", queueItem.Title, queueItem.Artist, min, sec), "", 0, nil)
  }
}

func addRandomSongsToQueue(ui *Ui) {
  response, _ := ui.connection.GetRandomSongs(20)
  for _, song := range response.RandomSongs.Songs {
    addSongToQueue(&song, ui)
  }
  updateQueueList(ui.player, ui.queueList)
}

func handleMpvEvents(ui *Ui) {
  for {
    e := <-ui.player.EventChannel
    if e == nil {
      break
      // we don't want to update anything if we're in the process of replacing the current track
    } else if e.Event_Id == mpv.EVENT_END_FILE && !ui.player.ReplaceInProgress {
      ui.startStopStatus.SetText("[::b]stmp: [red]stopped")
      // TODO it's gross that this is here, need better event handling
      if len(ui.player.Queue) > 0 {
        ui.player.Queue = ui.player.Queue[1:]
      }
      updateQueueList(ui.player, ui.queueList)
      ui.player.PlayNextTrack()
    } else if e.Event_Id == mpv.EVENT_START_FILE {
      ui.player.ReplaceInProgress = false
      ui.startStopStatus.SetText("[::b]stmp: [green]playing " + ui.player.Queue[0].Title)
      updateQueueList(ui.player, ui.queueList)
    }

    // TODO how to handle mpv errors here?
    position, _ := ui.player.Instance.GetProperty("time-pos", mpv.FORMAT_DOUBLE)
    // TODO only update these as needed
    duration, _ := ui.player.Instance.GetProperty("duration", mpv.FORMAT_DOUBLE)
    volume, _ := ui.player.Instance.GetProperty("volume", mpv.FORMAT_INT64)

    if position == nil {
      position = 0.0
    }

    if duration == nil {
      duration = 0.0
    }

    if volume == nil {
      volume = 0
    }

    ui.playerStatus.SetText(formatPlayerStatus(volume.(int64), position.(float64), duration.(float64)))
    ui.app.Draw()
  }
}

func formatPlayerStatus(volume int64, position float64, duration float64) string {
  if position < 0 {
    position = 0.0
  }

  if duration < 0 {
    duration = 0.0
  }

  positionMin, positionSec := secondsToMinAndSec(position)
  durationMin, durationSec := secondsToMinAndSec(duration)

  return fmt.Sprintf("[::b][%d%%][%02d:%02d/%02d:%02d]", volume,
  positionMin, positionSec, durationMin, durationSec)
}

func secondsToMinAndSec(seconds float64) (int, int) {
  minutes := math.Floor(seconds / 60)
  remainingSeconds := int(seconds) % 60
  return int(minutes), remainingSeconds
}

func iSecondsToMinAndSec(seconds int) (int, int) {
  minutes := seconds / 60
  remainingSeconds := seconds % 60
  return minutes, remainingSeconds
}

/// if the first argument isn't empty, return it, otherwise return the second
func stringOr(firstChoice string, secondChoice string) string {
  if firstChoice != "" {
    return firstChoice
  }
  return secondChoice
}

