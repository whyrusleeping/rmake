package rmake

import (

)

type Job struct {
	Command string
	Deps []string
	Output string
}
