package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrations(t *testing.T) {
	err := validateMigrations()
	assert.Nil(t, err)
}
