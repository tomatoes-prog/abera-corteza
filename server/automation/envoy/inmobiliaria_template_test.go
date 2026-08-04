// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package envoy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cortezaproject/corteza/server/automation/types"
	"github.com/cortezaproject/corteza/server/pkg/envoyx"
	"github.com/stretchr/testify/require"
)

func TestInmobiliariaWorkflowKeepsStepAndPathIDs(t *testing.T) {
	req := require.New(t)

	f, err := os.Open(filepath.Join(
		"..", "..", "..",
		"templates", "inmobiliaria-co", "provision", "workflows.yaml",
	))
	req.NoError(err)
	t.Cleanup(func() { req.NoError(f.Close()) })

	nodes, err := (YamlDecoder{}).Decode(context.Background(), envoyx.DecodeParams{
		Params: map[string]any{paramsKeyStream: f},
	})
	req.NoError(err)

	var workflow *types.Workflow
	for _, node := range nodes {
		if node.ResourceType == types.WorkflowResourceType {
			workflow = node.Resource.(*types.Workflow)
			break
		}
	}

	req.NotNil(workflow)
	req.Equal("asignar-lead-menor-carga", workflow.Handle)
	req.Len(workflow.Steps, 13)
	req.Len(workflow.Paths, 13)

	for _, step := range workflow.Steps {
		req.NotZero(step.ID, "workflow step %q lost its ID during YAML decoding", step.Meta.Name)
	}
	for _, path := range workflow.Paths {
		req.NotZero(path.ParentID, "workflow path lost its parent ID")
		req.NotZero(path.ChildID, "workflow path lost its child ID")
	}

	req.Equal(uint64(10), workflow.Steps[0].ID)
	req.Equal(uint64(12), workflow.Paths[0].ChildID)
}
