package controller

import (
	"github.com/jgwest/kouplet/pkg/controller/kouplettest"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, kouplettest.Add)
}
