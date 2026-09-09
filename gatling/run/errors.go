package run

// NotFoundError ends a search that found no run: the directory it names was read
// and held none, or was not there at all.
//
// It is not what a directory that could not be *read* returns. A failure to look
// is not an absence of runs, and reporting a broken mount or a permission as a
// clean "no runs here" is how a caller ends up debugging the wrong thing; that
// failure is returned wrapping its *fs.PathError instead.
type NotFoundError struct {
	// Dir is the directory that was searched. It is never empty, and it is the
	// caller's own path — Find never substitutes one, so there is nothing here a
	// caller did not ask for.
	Dir string
}

// Error names the directory that was searched.
func (e *NotFoundError) Error() string {
	return "gatling: no Gatling run under " + e.Dir
}
