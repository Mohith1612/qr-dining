package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets defence-in-depth response headers on every API response.
// The backend serves JSON only — it does not render HTML — so the CSP here is
// the strict API-shaped policy from security-hardening-checklist.md §9 with
// `default-src 'none'`. HTML pages are served by the Next.js frontend on a
// separate origin and carry their own CSP via next.config.ts.
//
// HSTS is gated on `enableHSTS`: enable only when the deployment terminates
// TLS in front of the API. In development the API is reached over plain
// HTTP and an HSTS header would lock the browser to HTTPS for a year.
func SecurityHeaders(enableHSTS bool) gin.HandlerFunc {
	const apiCSP = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", apiCSP)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=()")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Resource-Policy", "same-site")
		if enableHSTS {
			c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		c.Next()
	}
}
