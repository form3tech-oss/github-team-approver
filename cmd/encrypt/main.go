package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/form3tech-oss/github-team-approver/internal/api/aes"
)

// use to encrypt webhook tokens to place into your approver config
// example
// ./encrypt http://slack.com/bar ./test.key
// prints encrypted `http://slack.com/bar` using `test.key`, `test.key` should be hex encoded 256 bit
func main() {
	err := encrypt(os.Args)
	if err != nil {
		fmt.Printf("error encrypting hook, error: %v", err)
		os.Exit(-1)
	}
}

func encrypt(args []string) error {
	if len(os.Args) != 3 {
		return fmt.Errorf("you need to pass 3 arguments in format: encrypt http://slack.com/bar ./test.key")
	}
	webhook := args[1]
	keyPath := args[2]

	k, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("could not load key: error: %v", err)
	}

	key, err := hex.DecodeString(string(k))
	if err != nil {
		return fmt.Errorf("could not decode string from hex, error: %v", err)
	}

	c, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cryptor for encrypting: %v", err)
	}

	out, err := c.Encrypt(webhook)
	if err != nil {
		return fmt.Errorf("could not encrypt webhook, error: %v", err)
	}

	fmt.Printf(out)
	return nil
}
