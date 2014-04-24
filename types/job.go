package rmake

type Job struct {
	Command string
	Args    []string
	Deps    []string
	Output  string
	ID      int
}
