package fs

import "errors"

// ErrNoDefaultApp is returned by OpenWithDefault when the system has no
// application associated with the file type (and could not offer a chooser).
var ErrNoDefaultApp = errors.New("no application associated with this file type")
