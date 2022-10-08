package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
)

type Server struct {
	once sync.Once
	mux  *http.ServeMux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.once.Do(func() {
		if s.mux == nil {
			s.mux = http.NewServeMux()
		}
		// foo requires a login and a special header
		s.mux.HandleFunc("/foo", s.middlewareRequireToken(s.middlewareRequireHeader(s.handleFoo(), "SOME-HEADER")))
		// bar requires only a login
		s.mux.HandleFunc("/bar", s.middlewareRequireToken(s.handleBar()))
		s.mux.HandleFunc("/login", s.handleLogin())
	})
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleFoo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(&Foo{
			Foo: 123,
		})
		if err != nil {
			log.Println(err)
		}
	}
}

func (s *Server) handleBar() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(&Bar{
			Bar: "tiki",
		})
		if err != nil {
			log.Println(err)
		}
	}
}

func (s *Server) handleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		// this is the worst possible way to check auth
		if ok && user == "tom" && pass == "password1" {
			w.Header().Set("TOKEN", "YOU'RE SPECIAL")
			return
		}
		http.NotFound(w, r)
	}
}

func (s *Server) middlewareRequireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// this is also the worst possible auth
		if v := r.Header.Get("TOKEN"); !strings.EqualFold(v, "YOU'RE SPECIAL") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (s *Server) middlewareRequireHeader(next http.HandlerFunc, h string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get(h); v == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, err:=w.Write([]byte("missing header " + h))
			if err != nil {
				log.Println(err)
			}
			return
		}
		next(w, r)
	}
}
