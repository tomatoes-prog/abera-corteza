// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");

package envoy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cortezaproject/corteza/server/compose/types"
	"github.com/cortezaproject/corteza/server/pkg/envoyx"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

func TestGeometryPageBlockResolvesFeedModuleReferences(t *testing.T) {
	req := require.New(t)
	page := &types.Page{Blocks: types.PageBlocks{{
		Kind: "Geometry",
		Options: map[string]interface{}{
			"feeds": []interface{}{map[string]interface{}{
				"options": map[string]interface{}{"module": "inmuebles"},
			}},
		},
	}}}

	yamlRefs, _, err := (&auxYamlDoc{}).unmarshalPageBlocksNode(page, nil)
	req.NoError(err)
	req.Contains(yamlRefs, "Blocks.0.Options.feeds.0.ModuleID")
	req.Equal("inmuebles", yamlRefs["Blocks.0.Options.feeds.0.ModuleID"].Identifiers.FriendlyIdentifier())

	page.Blocks[0].Options["feeds"].([]interface{})[0].(map[string]interface{})["options"] = map[string]interface{}{"moduleID": "42"}
	storeRefs := decodePageRefs(page)
	req.Contains(storeRefs, "Blocks.0.Options.feeds.0.ModuleID")
	req.Equal("42", storeRefs["Blocks.0.Options.feeds.0.ModuleID"].Identifiers.FriendlyIdentifier())
}

func TestModuleMetadataSurvivesYamlDecode(t *testing.T) {
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

	for _, node := range nodes {
		if node.ResourceType != types.ModuleResourceType || node.Identifiers.FriendlyIdentifier() != "leads" {
			continue
		}

		module, ok := node.Resource.(*types.Module)
		req.True(ok)
		var meta map[string]any
		req.NoError(json.Unmarshal(module.Meta, &meta))
		req.NotEmpty(meta["description"])
		return
	}

	t.Fatal("module leads not found")
}

func TestModuleMetadataAcceptsExportedJSONText(t *testing.T) {
	req := require.New(t)
	r := &types.Module{}
	n := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!binary",
		Value: base64.StdEncoding.EncodeToString([]byte(`{"description":"Exported metadata"}`)),
	}

	_, _, err := (&auxYamlDoc{}).unmarshalModuleMetaNode(r, n)
	req.NoError(err)
	req.JSONEq(`{"description":"Exported metadata"}`, string(r.Meta))
}
