package http

import "net/http"

func Start(addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler}
	return srv.ListenAndServe()
}
