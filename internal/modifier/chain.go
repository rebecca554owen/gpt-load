package modifier

import (
	"sort"
)

type ModifierChain struct {
	modifiers []RequestBodyModifier
}

func NewModifierChain(modifiers ...RequestBodyModifier) *ModifierChain {
	sort.Slice(modifiers, func(i, j int) bool {
		return modifiers[i].Priority() < modifiers[j].Priority()
	})

	return &ModifierChain{
		modifiers: modifiers,
	}
}

func (mc *ModifierChain) Apply(ctx *ModificationContext, bodyBytes []byte) ([]byte, error) {
	var err error
	currentBytes := bodyBytes

	for _, modifier := range mc.modifiers {
		if modifier.ShouldApply(ctx) {
			currentBytes, _, err = modifier.Modify(ctx, currentBytes)
			if err != nil {
				return nil, err
			}
		}
	}

	return currentBytes, nil
}
