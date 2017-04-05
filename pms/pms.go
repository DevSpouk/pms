package pms

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ambientsound/pms/console"
	"github.com/ambientsound/pms/index"
	"github.com/ambientsound/pms/songlist"
	"github.com/ambientsound/pms/xdg"

	"github.com/fhs/gompd/mpd"
)

type PMS struct {
	MpdStatus        PlayerStatus
	MpdClient        *mpd.Client
	MpdClientWatcher *mpd.Watcher
	Index            *index.Index
	Library          *songlist.SongList

	host     string
	port     string
	password string

	libraryVersion int
	indexVersion   int

	EventLibrary chan int
	EventIndex   chan int
	EventPlayer  chan int
}

// PlayerStatus contains information about MPD's player status.
type PlayerStatus struct {
	Audio          string
	Bitrate        int
	Consume        bool
	Elapsed        float64
	Err            string
	MixRampDB      float64
	Playlist       int
	PlaylistLength int
	Random         bool
	Repeat         bool
	Single         bool
	Song           int
	SongID         int
	State          string
	Time           int
	Volume         int
}

// Strings found in the PlayerStatus.State variable.
const (
	StatePlay    string = "play"
	StateStop    string = "stop"
	StatePause   string = "pause"
	StateUnknown string = "unknown"
)

func createDirectory(dir string) error {
	dir_mode := os.ModeDir | 0755
	return os.MkdirAll(dir, dir_mode)
}

func makeAddress(host, port string) string {
	return fmt.Sprintf("%s:%s", host, port)
}

func indexDirectory(host, port string) string {
	cache_dir := xdg.CacheDirectory()
	index_dir := path.Join(cache_dir, host, port, "index")
	return index_dir
}

func indexStateFile(host, port string) string {
	cache_dir := xdg.CacheDirectory()
	state_file := path.Join(cache_dir, host, port, "state")
	return state_file
}

func (pms *PMS) writeIndexStateFile(version int) error {
	path := indexStateFile(pms.host, pms.port)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	str := fmt.Sprintf("%d\n", version)
	file.WriteString(str)
	return nil
}

func (pms *PMS) readIndexStateFile() (int, error) {
	path := indexStateFile(pms.host, pms.port)
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		version, err := strconv.Atoi(scanner.Text())
		if err != nil {
			return 0, err
		}
		return version, nil
	}

	return 0, fmt.Errorf("No data in index file")
}

func (pms *PMS) Connect(host, port, password string) error {
	pms.MpdClient = nil
	pms.host = host
	pms.port = port
	pms.password = password
	return pms.Reconnect()
}

func (pms *PMS) Reconnect() error {
	var err error
	addr := makeAddress(pms.host, pms.port)
	pms.MpdClient, err = mpd.DialAuthenticated(`tcp`, addr, pms.password)
	if err != nil {
		return err
	}
	pms.MpdClientWatcher, err = mpd.NewWatcher(`tcp`, addr, pms.password)
	if err != nil {
		pms.MpdClient.Close()
		return err
	}
	pms.UpdatePlayerStatus()
	go pms.watch()
	err = pms.Sync()
	return err
}

func (pms *PMS) watch() {
	for {
		select {
		case ev := <-pms.MpdClientWatcher.Error:
			console.Log("Watcher returned error: %s", ev)
			panic(ev)
		case ev, ok := <-pms.MpdClientWatcher.Event:
			console.Log("MPD IDLE: %s", ev)
			pms.UpdatePlayerStatus()
			if !ok {
				return
			}
		}
	}
}

// Sync retrieves the MPD library and stores it as a SongList in the
// PMS.Library variable. Furthermore, the search index is opened, and if it is
// older than the database version, a reindex task is started.
//
// If the SongList or Index is cached at the correct version, that part goes untouched.
func (pms *PMS) Sync() error {
	if pms.MpdClient == nil {
		return fmt.Errorf("Cannot call Sync() while not connected to MPD")
	}
	stats, err := pms.MpdClient.Stats()
	if err != nil {
		return fmt.Errorf("Error while retrieving library stats from MPD: %s", err)
	}

	libraryVersion, err := strconv.Atoi(stats["db_update"])
	console.Log("Sync(): server reports library version %d", libraryVersion)
	console.Log("Sync(): local version is %d", pms.libraryVersion)

	if libraryVersion != pms.libraryVersion {
		console.Log("Sync(): retrieving library from MPD...")
		library, err := pms.retrieveLibrary()
		if err != nil {
			return fmt.Errorf("Error while retrieving library from MPD: %s", err)
		}
		pms.Library = library
		pms.libraryVersion = libraryVersion
		console.Log("Sync(): local version updated to %d", pms.libraryVersion)
		pms.EventLibrary <- 1
	}

	console.Log("Sync(): opening search index")
	err = pms.openIndex()
	if err != nil {
		return fmt.Errorf("Error while opening index: %s", err)
	}
	console.Log("Sync(): index at version %d", pms.indexVersion)
	pms.EventIndex <- 1

	if libraryVersion != pms.indexVersion {
		console.Log("Sync(): index version differs from library version, reindexing...")
		pms.ReIndex()

		err = pms.writeIndexStateFile(pms.indexVersion)
		if err != nil {
			console.Log("Sync(): couldn't write index state file: %s", err)
		}
		console.Log("Sync(): index updated to version %d", pms.indexVersion)
	}

	console.Log("Sync(): finished.")

	return nil
}

func (pms *PMS) retrieveLibrary() (*songlist.SongList, error) {
	timer := time.Now()
	list, err := pms.MpdClient.ListAllInfo("/")
	if err != nil {
		return nil, err
	}
	console.Log("ListAllInfo in %s", time.Since(timer).String())

	s := songlist.NewFromAttrlist(list)
	s.Name = "Library"
	return s, nil
}

func (pms *PMS) openIndex() error {
	timer := time.Now()
	index_dir := indexDirectory(pms.host, pms.port)
	err := createDirectory(index_dir)
	if err != nil {
		return fmt.Errorf("Unable to create index directory %s!", index_dir)
	}

	pms.Index, err = index.New(index_dir, pms.Library)
	if err != nil {
		return fmt.Errorf("Unable to acquire index: %s", err)
	}

	pms.indexVersion, err = pms.readIndexStateFile()
	if err != nil {
		console.Log("Sync(): couldn't read index state file: %s", err)
	}

	console.Log("Opened index in %s", time.Since(timer).String())

	return nil
}

// UpdatePlayerStatus queries the MPD server for its status struct, and places
// a normalized copy in pms.MpdStatus.
func (pms *PMS) UpdatePlayerStatus() error {
	attrs, err := pms.MpdClient.Status()
	if err != nil {
		return err
	}

	console.Log("MPD player status: %s", attrs)

	pms.MpdStatus.Audio = attrs["audio"]
	pms.MpdStatus.Err = attrs["err"]
	pms.MpdStatus.State = attrs["state"]

	// The time field is divided into ELAPSED:LENGTH.
	// We only need the length field, since the elapsed field is sent as a
	// floating point value.
	split := strings.Split(attrs["time"], ":")
	if len(split) == 2 {
		pms.MpdStatus.Time, _ = strconv.Atoi(split[1])
	} else {
		pms.MpdStatus.Time = -1
	}

	pms.MpdStatus.Bitrate, _ = strconv.Atoi(attrs["bitrate"])
	pms.MpdStatus.Playlist, _ = strconv.Atoi(attrs["playlist"])
	pms.MpdStatus.PlaylistLength, _ = strconv.Atoi(attrs["playlistLength"])
	pms.MpdStatus.Song, _ = strconv.Atoi(attrs["song"])
	pms.MpdStatus.SongID, _ = strconv.Atoi(attrs["songID"])
	pms.MpdStatus.Volume, _ = strconv.Atoi(attrs["volume"])

	pms.MpdStatus.Elapsed, _ = strconv.ParseFloat(attrs["elapsed"], 64)
	pms.MpdStatus.MixRampDB, _ = strconv.ParseFloat(attrs["mixRampDB"], 64)

	pms.MpdStatus.Consume, _ = strconv.ParseBool(attrs["consume"])
	pms.MpdStatus.Random, _ = strconv.ParseBool(attrs["random"])
	pms.MpdStatus.Repeat, _ = strconv.ParseBool(attrs["repeat"])
	pms.MpdStatus.Single, _ = strconv.ParseBool(attrs["single"])

	pms.EventPlayer <- 0

	return nil
}

func (pms *PMS) ReIndex() {
	timer := time.Now()
	pms.Index.IndexFull()
	pms.indexVersion = pms.libraryVersion
	console.Log("Indexed songlist in %s", time.Since(timer).String())
}

func New() *PMS {
	pms := &PMS{}
	pms.EventLibrary = make(chan int)
	pms.EventIndex = make(chan int)
	pms.EventPlayer = make(chan int)
	return pms
}