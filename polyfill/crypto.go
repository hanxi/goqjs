package polyfill

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"

	"goqjs/qjs"
)

// InjectCrypto injects the crypto utility object into the global scope.
func InjectCrypto(ctx *qjs.Context) {
	global := ctx.GlobalObject()
	defer global.Free()

	crypto := ctx.NewObject()

	// crypto.md5(str) -> hex string
	crypto.SetFunction("md5", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError("md5 requires 1 argument")
		}
		var data []byte
		if buf := args[0].ToByteArray(); buf != nil {
			data = buf
		} else {
			data = []byte(args[0].String())
		}
		hash := md5.Sum(data)
		return ctx.NewString(hex.EncodeToString(hash[:]))
	}, 1)

	// crypto.randomBytes(size) -> ArrayBuffer
	crypto.SetFunction("randomBytes", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError("randomBytes requires 1 argument")
		}
		size := int(args[0].Int32())
		if size <= 0 {
			return ctx.NewArrayBuffer(nil)
		}
		buf := make([]byte, size)
		_, err := rand.Read(buf)
		if err != nil {
			return ctx.ThrowError(fmt.Sprintf("randomBytes: %v", err))
		}
		return ctx.NewArrayBuffer(buf)
	}, 1)

	// crypto.aesEncrypt(buffer, mode, key, iv) -> ArrayBuffer
	crypto.SetFunction("aesEncrypt", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 4 {
			return ctx.ThrowError("aesEncrypt requires 4 arguments: buffer, mode, key, iv")
		}

		var plaintext []byte
		if buf := args[0].ToByteArray(); buf != nil {
			plaintext = buf
		} else {
			plaintext = []byte(args[0].String())
		}

		mode := strings.ToLower(args[1].String())

		var key []byte
		if buf := args[2].ToByteArray(); buf != nil {
			key = buf
		} else {
			key = []byte(args[2].String())
		}

		var iv []byte
		if buf := args[3].ToByteArray(); buf != nil {
			iv = buf
		} else {
			iv = []byte(args[3].String())
		}

		block, err := aes.NewCipher(key)
		if err != nil {
			return ctx.ThrowError(fmt.Sprintf("aesEncrypt: %v", err))
		}

		// PKCS7 padding
		blockSize := block.BlockSize()
		padding := blockSize - len(plaintext)%blockSize
		padded := make([]byte, len(plaintext)+padding)
		copy(padded, plaintext)
		for i := len(plaintext); i < len(padded); i++ {
			padded[i] = byte(padding)
		}

		ciphertext := make([]byte, len(padded))

		switch mode {
		case "cbc":
			if len(iv) < blockSize {
				return ctx.ThrowError("aesEncrypt: IV too short for CBC mode")
			}
			encrypter := cipher.NewCBCEncrypter(block, iv[:blockSize])
			encrypter.CryptBlocks(ciphertext, padded)
		case "ecb":
			for i := 0; i < len(padded); i += blockSize {
				block.Encrypt(ciphertext[i:i+blockSize], padded[i:i+blockSize])
			}
		default:
			return ctx.ThrowError("aesEncrypt: unsupported mode: " + mode)
		}

		return ctx.NewArrayBuffer(ciphertext)
	}, 4)

	// crypto.rsaEncrypt(buffer, publicKeyPEM) -> ArrayBuffer
	crypto.SetFunction("rsaEncrypt", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 2 {
			return ctx.ThrowError("rsaEncrypt requires 2 arguments: buffer, publicKeyPEM")
		}

		var plaintext []byte
		if buf := args[0].ToByteArray(); buf != nil {
			plaintext = buf
		} else {
			plaintext = []byte(args[0].String())
		}

		keyPEM := args[1].String()
		block, _ := pem.Decode([]byte(keyPEM))
		if block == nil {
			return ctx.ThrowError("rsaEncrypt: invalid PEM")
		}

		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			// Try PKCS1
			pubKey, err2 := x509.ParsePKCS1PublicKey(block.Bytes)
			if err2 != nil {
				return ctx.ThrowError(fmt.Sprintf("rsaEncrypt: parse key: %v", err))
			}
			pub = pubKey
		}

		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return ctx.ThrowError("rsaEncrypt: not an RSA public key")
		}

		ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPub, plaintext)
		if err != nil {
			return ctx.ThrowError(fmt.Sprintf("rsaEncrypt: %v", err))
		}

		return ctx.NewArrayBuffer(ciphertext)
	}, 2)

	global.Set("crypto", crypto)
}
