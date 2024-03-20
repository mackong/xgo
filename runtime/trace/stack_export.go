package trace

import "time"

type RootExport struct {
	// current executed function
	Top      *StackExport
	Begin    time.Time
	Children []*StackExport
}

type StackExport struct {
	FuncInfo *FuncInfoExport

	Begin int64 // us
	End   int64 // us

	Args    interface{}
	Results interface{}
	Panic   bool
	Error   string

	Children []*StackExport
}

type FuncInfoExport struct {
	// FullName string
	Pkg          string
	IdentityName string
	Name         string
	RecvType     string
	RecvPtr      bool

	Generic bool

	RecvName string
	ArgNames []string
	ResNames []string

	// is first argument ctx
	FirstArgCtx bool
	// last last result error
	LastResultErr bool
}
