module github.com/aleDiPi/koboHomeAssistantDashboard

// Keep Go 1.23: Go 1.24+ needs Linux 3.2, the Kobo Glo runs an older kernel.
go 1.23.0

toolchain go1.23.12

require (
	golang.org/x/crypto/x509roots/fallback v0.0.0-20250305170421-49bf5b80c810
	golang.org/x/image v0.23.0
)

require golang.org/x/text v0.21.0 // indirect
