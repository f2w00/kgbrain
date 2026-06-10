package resource

import "database/sql"

type Module struct {
	Service *Service
	Handler *ResourceHandler
}

type ModuleDeps struct {
	StoreDB *sql.DB
}

func NewModule(deps ModuleDeps) (*Module, error) {
	repo, err := NewResourceRepo(deps.StoreDB)
	if err != nil {
		return nil, err
	}
	svc := NewService(repo)
	return &Module{
		Service: svc,
		Handler: NewResourceHandler(svc),
	}, nil
}
