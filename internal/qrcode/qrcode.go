package qrcode

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/skip2/go-qrcode"
)

// GenerateASCII generates a simple ASCII QR-like display for terminal
// For production, you'd use a real QR code library like github.com/skip2/go-qrcode
func GenerateASCII(url string) string {
	border := strings.Repeat("█", len(url)+4)
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════╗
║                     SCAN TO AUTHORIZE REQUESTS                    ║
╚══════════════════════════════════════════════════════════════════╝
%s
█  %s  █
%s

Open this URL on your phone to start approving/denying requests!
`, border, url, border)
}

func GenerateQrCode(url string) string {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		slog.Error("Error generating QR code", "error", err)
		return ""
	}
	return qr.ToSmallString(false)
}

// Note: For real QR codes, add this dependency:
// go get github.com/skip2/go-qrcode
// Then use:
// qr, _ := qrcode.New(url, qrcode.Medium)
// return qr.ToSmallString(false)

// Generate is an alias for GenerateASCII
func Generate(url string) string {
	return GenerateASCII(url) + "\n" + GenerateQrCode(url)
}
