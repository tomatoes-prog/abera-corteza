// Copyright 2026 Abera/Corteza contributors
// Licensed under the Apache License, Version 2.0.

package resource

import (
	"testing"

	"github.com/cortezaproject/corteza/server/compose/types"
	"github.com/stretchr/testify/require"
)

func TestComposePageGeometryFeedReferences(t *testing.T) {
	page := &types.Page{Blocks: types.PageBlocks{{
		Kind: "Geometry",
		Options: map[string]interface{}{
			"feeds": []interface{}{map[string]interface{}{
				"options": map[string]interface{}{"moduleID": "inmuebles"},
			}},
		},
	}}}

	namespaceRef := MakeRef(types.NamespaceResourceType, MakeIdentifiers("inmobiliaria-co"))
	resource := NewComposePage(page, namespaceRef, nil, nil)
	require.Len(t, resource.ModRefs, 1)
	require.Len(t, resource.BlockRefs[0], 1)
	require.Equal(t, "inmuebles", resource.ModRefs[0].Identifiers.First())
}
