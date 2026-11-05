package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/gliderlabs/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

func chadReplies(command string) (string, bool) {

	proceed := true
	msg := fmt.Sprintf("Yenna %s ?", command)

	if strings.HasPrefix(command, "ls") {
		msg = "Anga onnu illa keela potru"
	}

	if strings.HasPrefix(command, "cd") {
		msg = "Anga onnu illa keela potru"
	}

	if strings.HasPrefix(command, "pwd") {
		msg = "Anga onnu illa keela potru"
	}

	if strings.HasPrefix(command, "cat") {
		msg = "Anga onnu illa keela potru"
	}

	if strings.Contains(command, "bye") {
		msg = "Right uh varae"
		proceed = false
	}

	if strings.Contains(command, "exit") {
		msg = "Seri Kelambu"
		proceed = false
	}

	return msg + "\n\n", proceed
}

func main() {
	// 1. Define the session handler

	ssh.Handle(func(s ssh.Session) {
		io.WriteString(s, "Welcome to our minimal Go SSH server!\n")

		term := terminal.NewTerminal(s, "You        : ")

		for {
			line, err := term.ReadLine()
			if err != nil {
				break
			}

			msg, proceed := chadReplies(line)
			term.Write([]byte(msg))
			if !proceed {
				return
			}
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "2222"
	}
	addr := ":" + port

	// 2. Start the server and listen on a port
	log.Println("Starting SSH server on port: ", port)

	// log.Fatal(ssh.ListenAndServe(":2222", nil))

	log.Fatal(ssh.ListenAndServe(addr, nil))
}
