package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	// we can either run a server or a client

	switch {
	case len(os.Args) > 1 && strings.EqualFold(os.Args[1], "server"):
		s := &Server{}
		err := http.ListenAndServe("0.0.0.0:8000", s)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}

	case len(os.Args) > 1 && strings.EqualFold(os.Args[1], "client"):
		m, err := NewMyClient(http.DefaultClient, "http://127.0.0.1:8000")
		if err != nil {
			log.Fatal(err)
		}
		foo, err := m.Foo()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(foo)
		bar, err := m.Bar()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(bar)

	default:
		log.Println("expected a 'client' or 'server' argument")
		os.Exit(1)
	}
}
