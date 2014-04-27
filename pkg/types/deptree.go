package rmake

import (
	"fmt"
)

const (
	tBuild = iota
	tFile
)

type DepTreeNode struct {
	Result string
	DependsOn []*DepTreeNode
	Type int
}

func MakeDepTree(in *BuildPackage) (*DepTreeNode, error) {
	jobbyout := make(map[string]*Job)
	for _,j := range in.Jobs {
		jobbyout[j.Output] = j
	}

	final,ok := jobbyout[in.Output]
	if !ok {
		return fmt.Errorf("Could not find job for final output.")
	}
	delete(jobbyout, in.Output)

	root := new(DepTreeNode)
	root.Result = in.Output
	root.Type = tBuild
	err := root.Build(final.Deps, jobbyout, in.Files)
	if err != nil {
		return nil, err
	}

	return root,nil
}

func (t *DepTreeNode) Build(deps []string, jobs map[string]*Job, files map[string]*File) error {
	for _,d := range deps {
		j,ok := jobs[d]
		if ok {
			delete(jobs, d)
			node := new(DepTreeNode)
			node.Result = d
			node.Type = tBuild
			if err := node.Build(j.Deps, jobs, files); err != nil {
				return err
			}
			t.DependsOn = append(t.DependsOn, node)
			continue
		}

		fi,ok := files[d]
		if ok {
			node := new(DepTreeNode)
			node.Result = d
			node.Type = tFile
			t.DependsOn = append(t.DependsOn, node)
			continue
		}

		return fmt.Errorf("Could resolve dependency '%s' for job '%s'", d, t.Result)
	}
}
