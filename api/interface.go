package api

import (
	"github.com/ambientsound/pms/songlist"
)

type SonglistWidget interface {
	AddSonglist(songlist.Songlist)
	FallbackSonglist() songlist.Songlist
	RemoveSonglist(int) error
	SetSonglist(songlist.Songlist)
	SetSonglistIndex(int) error
	Size() (int, int)
	Songlist() songlist.Songlist
	SonglistIndex() (int, error)
	SonglistsLen() int
	ValidSonglistIndex(int) bool
}

type MultibarWidget interface {
	Mode() int
	SetMode(int) error
}

type UI interface {
	PostFunc(func())
	Refresh()
}
