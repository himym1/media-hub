package strm

import "errors"

var (
	ErrAuthExpired    = errors.New("115 session is unauthorized")
	ErrListFailed     = errors.New("unable to list 115 files for STRM sync")
	ErrPathUnwritable = errors.New("STRM target path is not writable")
	ErrInvalidRequest = errors.New("STRM sync request is invalid")
	ErrNoVideos       = errors.New("transferred folder has no video files")
	ErrBusy           = errors.New("STRM sync is already running")
)
