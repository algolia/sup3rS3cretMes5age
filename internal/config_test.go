package internal

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	_ = os.Setenv(HttpBindingAddressVarenv, ":8080")
	_ = os.Setenv(VaultPrefixenv, "cubbyhole/")
	_ = os.Setenv(AllowedOriginsVarenv, "http://localhost,https://example.com")
	defer func() {
		_ = os.Unsetenv(HttpBindingAddressVarenv)
		_ = os.Unsetenv(VaultPrefixenv)
		_ = os.Unsetenv(AllowedOriginsVarenv)
	}()

	cnf := LoadConfig()

	assert.Equal(t, ":8080", cnf.HttpBindingAddress)
	assert.Equal(t, "cubbyhole/", cnf.VaultPrefix)
	assert.False(t, cnf.HttpsRedirectEnabled)
	assert.Equal(t, []string{"http://localhost", "https://example.com"}, cnf.AllowedOrigins)
}
