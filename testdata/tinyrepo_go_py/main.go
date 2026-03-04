package main

import (
	"fmt"
	"net/http"
)

type Server struct{ Addr string }

func (s Server) URL() string { return "http://" + s.Addr }

func main() {
	_ = http.MethodGet
	fmt.Println(Server{Addr: "127.0.0.1:8080"}.URL())
}
