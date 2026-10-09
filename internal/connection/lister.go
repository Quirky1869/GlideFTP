package connection

// Lister lists directories for one worker of a parallel remote search.
type Lister interface {
	ListDir(path string) ([]RemoteFileEntry, error)
	Close() error
}

// ParallelLister is implemented by clients that can hand out extra listers,
// so a recursive search can list several folders at the same time instead of
// one LIST round-trip after another.
type ParallelLister interface {
	OpenLister() (Lister, error)
}
