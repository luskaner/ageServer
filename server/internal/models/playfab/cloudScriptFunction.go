package playfab

import (
	"github.com/luskaner/ageServer/server/internal/models"
)

type Named interface {
	Name() string
}

type CloudScriptFunction interface {
	Named
	Run(game models.Game, user models.User, parameters any) any
	NewParameters() any
}

type SpecificCloudScriptFunction[P any, R any] interface {
	Named
	RunTyped(game models.Game, user models.User, parameters *P) *R
}

type CloudScriptFunctionBase[P any, R any] struct {
	SpecificCloudScriptFunction[P, R]
}

func NewCloudScriptFunctionBase[P any, R any](impl SpecificCloudScriptFunction[P, R]) *CloudScriptFunctionBase[P, R] {
	return &CloudScriptFunctionBase[P, R]{SpecificCloudScriptFunction: impl}
}

func (c *CloudScriptFunctionBase[P, R]) NewParameters() any {
	return new(P)
}

func (c *CloudScriptFunctionBase[P, R]) Run(game models.Game, user models.User, parameters any) any {
	return c.RunTyped(game, user, parameters.(*P))
}
