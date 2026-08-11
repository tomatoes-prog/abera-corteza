// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");

package envoy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cortezaproject/corteza/server/compose/types"
	"github.com/cortezaproject/corteza/server/pkg/envoyx"
	"github.com/stretchr/testify/require"
)

func TestModuleFieldsStayAttachedToTheirModule(t *testing.T) {
	req := require.New(t)
	file, err := os.Open(filepath.Join(
		"..", "..", "..",
		"templates", "inmobiliaria-co", "provision", "modules.yaml",
	))
	req.NoError(err)
	t.Cleanup(func() { req.NoError(file.Close()) })

	nodes, err := (YamlDecoder{}).Decode(context.Background(), envoyx.DecodeParams{
		Params: map[string]any{paramsKeyStream: file},
	})
	req.NoError(err)

	graph := envoyx.BuildDepGraph(nodes...)
	fieldsByModule := make(map[string][]string)
	for _, node := range nodes {
		if node.ResourceType != types.ModuleResourceType {
			continue
		}
		children := graph.ChildrenForResourceType(node, types.ModuleFieldResourceType)
		for _, field := range ownedModuleFieldNodes(node, children) {
			fieldsByModule[node.Identifiers.FriendlyIdentifier()] = append(
				fieldsByModule[node.Identifiers.FriendlyIdentifier()],
				field.Identifiers.FriendlyIdentifier(),
			)
		}
	}

	req.NotContains(fieldsByModule["leads"], "lead")
	req.Contains(fieldsByModule["citas"], "lead")
	req.Contains(fieldsByModule["actividades"], "lead")
	req.Contains(fieldsByModule["negociaciones"], "lead")
}
