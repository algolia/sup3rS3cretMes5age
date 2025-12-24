package internal

import (
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestSetupRoutes(t *testing.T) {

	/*
		expectRoutes := []*echo.Route{
			*echo.Route{Method: echo.GET, Path: "/"},
			*echo.Route{Method: echo.POST, Path: "/"},
			*echo.Route{Method: echo.GET, Path: "/msg", Name:"},
			*echo.Route{Method: echo.POST, Path: "/"},
		}
	*/
	e := echo.New()
	handlers := newSecretHandlers(&FakeSecretMsgStorer{})

	setupRoutes(e, handlers)

	// Verify routes are registered
	routes := e.Routes()
	if assert.NotEmpty(t, routes) {
		// There should be 18 routes registered:
		// GET / (redirect)
		// GET /robots.txt
		// ANY /health (11)
		// GET /secret
		// POST /secret
		// GET /msg
		// GET /getmsg
		// GET /static*
		assert.Equal(t, 18, len(routes))
	}
}

func TestSetupMiddlewares(t *testing.T) {
	e := echo.New()
	cnf := conf{
		HttpBindingAddress: ":8080",
		VaultPrefix:        "cubbyhole/",
		AllowedOrigins:     []string{"*"},
	}

	setupMiddlewares(e, cnf)

	// Smoke test - just verify it doesn't panic
	assert.NotNil(t, e)
}
