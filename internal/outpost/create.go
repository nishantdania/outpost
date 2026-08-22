package outpost

import "context"

type Result struct {
	Message string
}

func Create(context.Context) (Result, error) {
	return Result{Message: "Hello, World!"}, nil
}
