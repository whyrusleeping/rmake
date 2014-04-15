package rmake

import (

)

type Job struct {
	Command string
	Args []string
	Deps []string
	Output string
}
