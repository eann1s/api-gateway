package http_public


type Deps struct {
}

type Handlers struct {
	deps Deps
}

func NewHandlers(deps Deps) *Handlers {
	return &Handlers{
		deps: deps,
	}
}
