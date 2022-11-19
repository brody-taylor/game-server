package httpserver

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

const port = "8080"

func Run() error {
	// Register endpoints
	http.HandleFunc("/helloworld", helloWorld)

	// Start listening for requests
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil && errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func helloWorld(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("got /helloworld request\n")
	io.WriteString(w, "Hello world!\n")
}
