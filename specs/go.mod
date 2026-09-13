module github.com/galax-io/parsec/specs

// Not a module anyone imports. The file is here because the Go module zip
// excludes any directory that has one, and specs has no Go in it: a consumer
// pulling the library has no use for it and should not pay to download it.
// go build, go vet and go test are unaffected — there are no packages here.
go 1.25
