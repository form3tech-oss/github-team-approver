package aes

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {

	key, _ := hex.DecodeString("6368616e676520746869732070617373776f726420746f206120736563726574")
	plainText := "http://slack.com/1234/XXXX"

	c, err := New(key)
	if err != nil {
		t.Errorf("could not create crytor, error: %v", err)
	}

	cipherText, err := c.Encrypt(plainText)
	if err != nil {
		t.Errorf("could not encrypt text, error: %v", err)
	}

	decryptedText, err := c.Decrypt(cipherText)
	if err != nil {
		t.Errorf("could not decrypt cipher text, error: %v", err)
	}

	if !strings.EqualFold(plainText, decryptedText) {
		t.Errorf("decryption error, expected: %s, got: %s", plainText, decryptedText)
	}
}

func TestDecrypt(t *testing.T) {
	key, _ := hex.DecodeString("6368616e676520746869732070617373776f726420746f206120736563726574")
	plainText := "http://slack.com/1234/XXXX"

	c, err := New(key)
	if err != nil {
		t.Errorf("could not create crytor, error: %v", err)
	}

	decryptedText, err := c.Decrypt("8gqFxjlwhBM768-O6yxcSf23TKGPP9IbmbhTQMhDHgWgP_0rY9Me1nBvKEUBF9YH")
	if err != nil {
		t.Errorf("could not decrypt cipher text, error: %v", err)
	}

	if !strings.EqualFold(plainText, decryptedText) {
		t.Errorf("decryption error, expected: %s, got: %s", plainText, decryptedText)
	}
}
