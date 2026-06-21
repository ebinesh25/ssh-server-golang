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

func chadReplies(command string) string {

	msg := fmt.Sprintf("Yenna %s ?", command)

	if strings.HasPrefix(command, "ls") {
		msg = "Anga onnu illa keela potru"
	}

	return msg + "\n\n"
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

			if strings.Trim(line, "") == "exit" {
				return
			}

			term.Write([]byte(chadReplies(line)))
		}
	})

	// 2. Start the server and listen on a port
	log.Println("Starting SSH server on port 2222...")

	// log.Fatal(ssh.ListenAndServe(":2222", nil))
	// replace log.Fatal(ssh.ListenAndServe(":2222", nil))
	addr := ":" + os.Getenv("PORT")
	log.Fatal(ssh.ListenAndServe(addr, nil))
}
