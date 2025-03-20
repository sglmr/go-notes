// This is a CLI tool for generating the argon2id encoded hash of a password.
package main

import (
	"fmt"
	"log"
	"syscall"

	"github.com/sglmr/gowebstart/internal/argon2id"
	"golang.org/x/term"
)

func main() {
	// Try to read password securely first (won't echo characters)
	fmt.Print("   Enter password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalln("could not get password: %w", err)
	}

	// Ask to re-enter password to confirm
	fmt.Print("\nRe-Enter password: ")
	passwordCheck, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalln("could not get re-entered password: %w", err)
	}

	// Exit if the passwords don't match
	if string(password) != string(passwordCheck) {
		log.Fatalln("passwords don't match")
	}

	// Generate an argon2id hash
	encodedHash, err := argon2id.CreateHash(string(password), argon2id.DefaultParams)
	if err != nil {
		log.Fatalln("Error generating hash:", err)
	}

	// Print the resulting hash
	fmt.Println("\n\tPassword hash:", string(encodedHash))
}
