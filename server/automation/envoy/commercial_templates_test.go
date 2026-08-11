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
	"strings"
	"testing"

	"github.com/cortezaproject/corteza/server/automation/types"
	"github.com/cortezaproject/corteza/server/pkg/envoyx"
	"github.com/cortezaproject/corteza/server/pkg/expr"
	"github.com/stretchr/testify/require"
)

func TestCommercialTemplatesDecode(t *testing.T) {
	templates := []string{
		"inmobiliaria-co",
		"automotriz-co",
		"admisiones-educativas-co",
		"servicios-tecnicos-co",
		"centro-contacto-co",
		"soporte-renovaciones-co",
	}

	for _, template := range templates {
		template := template
		t.Run(template, func(t *testing.T) {
			req := require.New(t)
			provisionDir := filepath.Join(
				"..", "..", "..", "templates", template, "provision",
			)

			entries, err := os.ReadDir(provisionDir)
			req.NoError(err)
			req.NotEmpty(entries)

			decodedWorkflows := 0
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
					continue
				}

				f, err := os.Open(filepath.Join(provisionDir, entry.Name()))
				req.NoError(err)

				nodes, decodeErr := (YamlDecoder{}).Decode(
					context.Background(),
					envoyx.DecodeParams{
						Params: map[string]any{paramsKeyStream: f},
					},
				)
				req.NoError(f.Close())
				req.NoError(decodeErr, entry.Name())

				for _, node := range nodes {
					if node.ResourceType != types.WorkflowResourceType {
						continue
					}

					workflow := node.Resource.(*types.Workflow)
					decodedWorkflows++
					req.NotEmpty(workflow.Handle)
					req.True(workflow.Enabled, workflow.Handle)
					req.NotEmpty(workflow.Steps, workflow.Handle)
					ensureWorkflowVisuals(workflow)
					assertWorkflowVisuals(t, workflow)

					hasRoleIterator := false
					for _, step := range workflow.Steps {
						req.NotZero(
							step.ID,
							"workflow %q step %q lost its ID",
							workflow.Handle,
							step.Meta.Name,
						)
						hasRoleIterator = hasRoleIterator || step.Ref == "rolesEachMember"
						for _, expression := range append(step.Arguments, step.Results...) {
							if expression.Expr == "" {
								continue
							}
							_, err := expr.NewParser().Parse(expression.Expr)
							req.NoError(err, "workflow %q has an invalid step expression", workflow.Handle)
						}
					}
					if hasRoleIterator {
						req.Contains(
							node.References,
							"RunAs",
							"workflow %q must use its template automation identity",
							workflow.Handle,
						)
					}
					for _, path := range workflow.Paths {
						req.NotZero(path.ParentID, workflow.Handle)
						req.NotZero(path.ChildID, workflow.Handle)
						req.True(hasVisualFields(path.Meta.Visual, "id", "parent"), workflow.Handle)
						if path.Expr != "" {
							_, err := expr.NewParser().Parse(path.Expr)
							req.NoError(err, "workflow %q has an invalid path expression", workflow.Handle)
						}
					}
					for _, step := range workflow.Steps {
						req.True(hasVisualFields(step.Meta.Visual, "id", "xywh", "parent"), workflow.Handle)
					}
				}
			}

			req.NotZero(decodedWorkflows)
		})
	}
}
