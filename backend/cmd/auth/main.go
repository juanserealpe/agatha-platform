// Command auth genera los secretos de la autenticación. No toca la base ni el
// .env: imprime el valor y se copia a mano.
//
//	go run ./cmd/auth -hash-password   lee la contraseña de stdin e imprime AGATHA_ADMIN_PASSWORD_HASH
//	go run ./cmd/auth -generate-key    imprime un valor aleatorio para AGATHA_MOBILE_API_KEYS o AGATHA_JWT_SECRET
//
// La contraseña se lee de stdin (no de un flag) para que no quede en el
// historial de la shell: printf '%s' 'la-clave' | go run ./cmd/auth -hash-password
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/auth/infrastructure/security"
)

const (
	hashCost     = 12
	minPassword  = 12
	randomKeyLen = 32
)

func main() {
	hashPassword := flag.Bool("hash-password", false, "lee la contraseña de stdin e imprime AGATHA_ADMIN_PASSWORD_HASH")
	generateKey := flag.Bool("generate-key", false, "imprime 32 bytes aleatorios en base64url (key móvil o secreto JWT)")
	flag.Parse()

	switch {
	case *hashPassword:
		os.Exit(runHashPassword())
	case *generateKey:
		os.Exit(runGenerateKey())
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func runHashPassword() int {
	fmt.Fprintln(os.Stderr, "Contraseña (se lee una línea de stdin):")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(os.Stderr, "no se pudo leer la contraseña:", err)
		return 1
	}
	password := strings.TrimRight(line, "\r\n")
	if len(password) < minPassword {
		fmt.Fprintf(os.Stderr, "la contraseña debe tener al menos %d caracteres\n", minPassword)
		return 1
	}

	encoded, err := security.HashPassword(password, hashCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo generar el hash:", err)
		return 1
	}
	fmt.Println("AGATHA_ADMIN_PASSWORD_HASH=" + encoded)
	return 0
}

func runGenerateKey() int {
	buf := make([]byte, randomKeyLen)
	if _, err := rand.Read(buf); err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo generar la key:", err)
		return 1
	}
	fmt.Println(base64.RawURLEncoding.EncodeToString(buf))
	return 0
}
