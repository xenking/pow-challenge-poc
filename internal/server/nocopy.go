package server

// Embed this type into a struct, which mustn't be copied,
// so `go vet` gives a warning if this struct is copied.
//
// See https://stackoverflow.com/questions/52494458/nocopy-minimal-example
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
