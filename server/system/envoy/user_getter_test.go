// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");

package envoy

import (
	"context"
	"testing"

	"github.com/cortezaproject/corteza/server/pkg/envoyx"
	"github.com/cortezaproject/corteza/server/system/types"
	"github.com/stretchr/testify/require"
)

func TestUserGetterPrefersExactProvisioningReference(t *testing.T) {
	user := &envoyx.Node{
		Resource:     &types.User{ID: 42},
		ResourceType: types.UserResourceType,
		Identifiers:  envoyx.MakeIdentifiers("asesor-demo"),
	}
	graph := envoyx.BuildDepGraph(user)

	resolved, err := MakeUserGetter(nil, graph).Resolve(context.Background(), "asesor-demo")
	require.NoError(t, err)
	require.Equal(t, uint64(42), resolved)
}
