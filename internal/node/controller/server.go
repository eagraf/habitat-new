package controller

type Server struct {
	controller NodeController
}

func NewServer(ctr NodeController) *Server {
	return &Server{
		controller: ctr,
	}
}
