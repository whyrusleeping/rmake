package rmake

import (
	"fmt"
)

const (
	TBuild = iota
	TFile
)

type DepTreeNode struct {
	Result string
	DependsOn []*DepTreeNode
	Type int
}

//For use on the manager
func MakeDepTreeBP(in *BuildPackage) (*DepTreeNode, error) {
	jobbyout := make(map[string]*Job)
	for _,j := range in.Jobs {
		jobbyout[j.Output] = j
	}

	final,ok := jobbyout[in.Output]
	if !ok {
		return nil,fmt.Errorf("Could not find job for final output.")
	}
	delete(jobbyout, in.Output)

	fi := make(map[string]bool)
	for fn,_ := range in.Files {
		fi[fn] = true
	}
	root := new(DepTreeNode)
	root.Result = in.Output
	root.Type = TBuild
	err := root.Build(final.Deps, jobbyout, fi)
	if err != nil {
		return nil, err
	}

	return root,nil
}


func (t *DepTreeNode) Build(deps []string, jobs map[string]*Job, files map[string]bool) error {
	for _,d := range deps {
		j,ok := jobs[d]
		if ok {
			delete(jobs, d)
			node := new(DepTreeNode)
			node.Result = d
			node.Type = TBuild
			if err := node.Build(j.Deps, jobs, files); err != nil {
				return err
			}
			t.DependsOn = append(t.DependsOn, node)
			continue
		}

		_,ok = files[d]
		if ok {
			node := new(DepTreeNode)
			node.Result = d
			node.Type = TFile
			t.DependsOn = append(t.DependsOn, node)
			continue
		}

		return fmt.Errorf("Could not resolve dependency '%s' for job '%s'", d, t.Result)
	}
	return nil
}

func (t *DepTreeNode) Print() {
	fmt.Printf("Final job %s depends on:\n", t.Result)
	for _,dep := range t.DependsOn {
		dep.rPrint(1)
	}
}

func (t *DepTreeNode) rPrint(depth int) {
	for i := 0; i < depth; i++ {
		fmt.Print("\t");
	}
	fmt.Printf("%s depends on %d items:\n", t.Result, len(t.DependsOn))
	for _,dep := range t.DependsOn {
		dep.rPrint(depth+1)
	}
}
