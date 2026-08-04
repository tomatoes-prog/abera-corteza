// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");

package envoy

import (
	"context"
	"strings"
	"testing"

	"github.com/cortezaproject/corteza/server/pkg/envoyx"
	"github.com/cortezaproject/corteza/server/pkg/envoyx/datasource"
	"github.com/stretchr/testify/require"
)

func TestDecodeStandaloneRecordSource(t *testing.T) {
	req := require.New(t)
	nodes, err := (YamlDecoder{}).Decode(context.Background(), envoyx.DecodeParams{
		Params: map[string]any{
			paramsKeyStream: strings.NewReader(`
record_sources:
  - source: leads.csv
    key: ID
    references:
      namespace: inmobiliaria-co
      module: leads
      inmueblesInteres.module: inmuebles
      inmueblesInteres.datasource: inmuebles
    defaultable: true
    map:
      ID: { column: ID, skip: true }
`),
		},
	})
	req.NoError(err)
	req.Len(nodes, 1)

	node := nodes[0]
	req.Equal(ComposeRecordDatasourceAuxType, node.ResourceType)
	req.Equal("leads", node.Identifiers.FriendlyIdentifier())
	req.Equal("inmobiliaria-co", node.Scope.Identifiers.FriendlyIdentifier())
	req.Equal("leads", node.References["ModuleID"].Identifiers.FriendlyIdentifier())
	req.Equal("inmuebles", node.References["inmueblesInteres.module"].Identifiers.FriendlyIdentifier())
	req.Equal(
		ComposeRecordDatasourceAuxType,
		node.References["inmueblesInteres.datasource"].ResourceType,
	)
}

func TestRecordMakerPreservesMultipleRecordReferences(t *testing.T) {
	req := require.New(t)
	related := &RecordDatasource{
		refToID: map[string]uint64{
			"inmueble_001": 101,
			"inmueble_002": 102,
		},
	}
	getters := map[string]*recordGetter{
		"inmuebles": {relDatasource: related},
	}

	makeRecord := (StoreEncoder{}).recordMaker(nil, nil, getters, nil)
	record, err := makeRecord(context.Background(), datasource.RawRecord{
		"inmuebles": {
			Name:   "inmuebles",
			Values: []string{"inmueble-001", "inmueble-002"},
		},
	})
	req.NoError(err)
	req.Len(record.Values.FilterByName("inmuebles"), 2)
	req.Equal("101", record.Values.Get("inmuebles", 0).Value)
	req.Equal("102", record.Values.Get("inmuebles", 1).Value)
}
