// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package envoy

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cortezaproject/corteza/server/automation/types"
	"github.com/stretchr/testify/require"
)

func TestEnsureWorkflowVisualsPreservesExistingLayout(t *testing.T) {
	req := require.New(t)

	wf := &types.Workflow{
		Steps: types.WorkflowStepSet{
			&types.WorkflowStep{ID: 10, Kind: types.WorkflowStepKindGateway, Meta: types.WorkflowStepMeta{Name: "Inicio"}},
			&types.WorkflowStep{
				ID:   20,
				Kind: types.WorkflowStepKindTermination,
				Meta: types.WorkflowStepMeta{Visual: map[string]interface{}{
					"id":     "20",
					"xywh":   []float64{640, 80, 220, 72},
					"parent": workflowVisualRootID,
					"layout": workflowVisualLayoutID,
				}},
			},
		},
		Paths: types.WorkflowPathSet{
			&types.WorkflowPath{ParentID: 10, ChildID: 20},
		},
	}

	req.True(ensureWorkflowVisuals(wf))
	req.Equal("10", wf.Steps[0].Meta.Visual["id"])
	req.Equal([]float64{640, 80, 220, 72}, wf.Steps[1].Meta.Visual["xywh"])
	req.Equal("path-10-20-0", wf.Paths[0].Meta.Visual["id"])
	req.Contains(wf.Paths[0].Meta.Visual["style"], "exitX=1")
	req.Contains(wf.Paths[0].Meta.Visual["style"], "entryX=0")
	req.Equal(workflowVisualEdgeStyleID, wf.Paths[0].Meta.Visual["edgeStyleID"])

	req.False(ensureWorkflowVisuals(wf), "a second pass should not rewrite an existing layout")
}

func TestEnsureWorkflowVisualsMigratesV2Layout(t *testing.T) {
	req := require.New(t)
	wf := &types.Workflow{
		Steps: types.WorkflowStepSet{
			&types.WorkflowStep{ID: 10, Kind: types.WorkflowStepKindGateway, Meta: types.WorkflowStepMeta{Visual: map[string]interface{}{
				"id": "10", "xywh": []float64{10, 10, 200, 80}, "parent": workflowVisualRootID, "layout": workflowVisualLayoutV2,
			}}},
			&types.WorkflowStep{ID: 20, Kind: types.WorkflowStepKindTermination, Meta: types.WorkflowStepMeta{Visual: map[string]interface{}{
				"id": "20", "xywh": []float64{20, 20, 200, 80}, "parent": workflowVisualRootID, "layout": workflowVisualLayoutV2,
			}}},
		},
		Paths: types.WorkflowPathSet{&types.WorkflowPath{ParentID: 10, ChildID: 20, Meta: types.WorkflowPathMeta{Visual: map[string]interface{}{
			"id": "old-path", "parent": workflowVisualRootID, "layout": workflowVisualLayoutV2,
		}}}},
	}

	req.True(ensureWorkflowVisuals(wf))
	req.Equal(workflowVisualLayoutID, wf.Steps[0].Meta.Visual["layout"])
	req.Equal(workflowVisualLayoutID, wf.Paths[0].Meta.Visual["layout"])
	req.Equal("old-path", wf.Paths[0].Meta.Visual["id"])
	req.Contains(wf.Paths[0].Meta.Visual["style"], "exitY=0.500")
}

func TestEnsureWorkflowVisualsSeparatesBranchesAndAnchorsRoutes(t *testing.T) {
	req := require.New(t)
	wf := &types.Workflow{
		Steps: types.WorkflowStepSet{
			&types.WorkflowStep{ID: 1}, &types.WorkflowStep{ID: 2},
			&types.WorkflowStep{ID: 3}, &types.WorkflowStep{ID: 4},
		},
		Paths: types.WorkflowPathSet{
			&types.WorkflowPath{ParentID: 1, ChildID: 2},
			&types.WorkflowPath{ParentID: 1, ChildID: 3},
			&types.WorkflowPath{ParentID: 2, ChildID: 4},
			&types.WorkflowPath{ParentID: 3, ChildID: 4},
		},
	}

	req.True(ensureWorkflowVisuals(wf))
	assertWorkflowVisuals(t, wf)
	req.Contains(wf.Paths[0].Meta.Visual["style"], "exitY=0.180")
	req.Contains(wf.Paths[1].Meta.Visual["style"], "exitY=0.820")
	req.Contains(wf.Paths[2].Meta.Visual["style"], "entryY=0.180")
	req.Contains(wf.Paths[3].Meta.Visual["style"], "entryY=0.820")
}

func TestEnsureTriggerVisual(t *testing.T) {
	req := require.New(t)

	trigger := &types.Trigger{
		ID:           42,
		StepID:       10,
		ResourceType: "compose:record",
		EventType:    "beforeCreate",
	}

	req.True(ensureTriggerVisual(trigger, map[uint64][]float64{10: {80, 80}}, 0, 1))
	req.NotNil(trigger.Meta)
	req.True(hasTriggerVisual(trigger.Meta))
	req.Equal("trigger-42", trigger.Meta.Visual["id"])

	edges, ok := trigger.Meta.Visual["edges"].([]interface{})
	req.True(ok)
	req.Len(edges, 1)
	edge := edges[0].(map[string]interface{})
	req.Equal("trigger-42", edge["parentID"])
	req.Equal("10", edge["childID"])
	edgeVisual := edge["meta"].(map[string]interface{})["visual"].(map[string]interface{})
	req.Contains(edgeVisual["style"], "exitX=1")
	req.Contains(edgeVisual["style"], "entryX=0")

	req.False(ensureTriggerVisual(trigger, map[uint64][]float64{10: {80, 80}}, 0, 1))
}

func TestRepairWorkflowEdgeVisualPreservesRoute(t *testing.T) {
	req := require.New(t)

	visual := map[string]interface{}{
		"id":     "path-10-20-0",
		"parent": workflowVisualRootID,
		"points": []interface{}{map[string]interface{}{"x": float64(400), "y": float64(200)}},
		"style":  workflowVisualLegacyEdgeStyle,
		"value":  "old label",
	}

	req.True(repairWorkflowEdgeVisual(visual, "Condición", 0.18, 0.82))
	req.Contains(visual["style"], "exitY=0.180")
	req.Contains(visual["style"], "entryY=0.820")
	req.Equal(workflowVisualEdgeStyleID, visual["edgeStyleID"])
	req.Equal("Condición", visual["value"])
	req.Equal([]interface{}{map[string]interface{}{"x": float64(400), "y": float64(200)}}, visual["points"])
	req.False(repairWorkflowEdgeVisual(visual, "Condición", 0.18, 0.82))
}

func TestRepairWorkflowNodeVisualMigratesLegacyDimensions(t *testing.T) {
	req := require.New(t)

	visual := map[string]interface{}{
		"id":     "10",
		"xywh":   []float64{320, 80, workflowVisualPreviousNodeW, workflowVisualPreviousNodeH},
		"parent": workflowVisualRootID,
	}

	req.True(repairWorkflowNodeVisual(visual))
	req.Equal([]float64{320, 80, workflowVisualNodeW, workflowVisualNodeH}, visual["xywh"])
	req.False(repairWorkflowNodeVisual(visual))
}

func assertWorkflowVisuals(t *testing.T, wf *types.Workflow) {
	t.Helper()
	req := require.New(t)
	nodes := make(map[string]*types.WorkflowStep)
	rects := make(map[string][]float64)
	for _, step := range wf.Steps {
		req.NotNil(step)
		visual := step.Meta.Visual
		id, ok := visual["id"].(string)
		req.True(ok && id != "", "step %d has no visual id", step.ID)
		req.NotContains(nodes, id, "duplicate visual id %q", id)
		nodes[id] = step
		rect, ok := visualXYWH(visual)
		req.True(ok, "step %d has invalid xywh", step.ID)
		rects[id] = rect
	}

	for leftID, left := range rects {
		for rightID, right := range rects {
			if leftID >= rightID {
				continue
			}
			req.False(rectanglesOverlap(left, right), "%s overlaps %s", leftID, rightID)
		}
	}

	pathIDs := make(map[string]bool)
	for index, path := range wf.Paths {
		req.NotNil(path)
		req.NotEqual(path.ParentID, path.ChildID, "path %d is a self-loop", index)
		parentID := fmt.Sprint(path.ParentID)
		childID := fmt.Sprint(path.ChildID)
		req.Contains(nodes, parentID, "path %d has missing source", index)
		req.Contains(nodes, childID, "path %d has missing target", index)
		visual := path.Meta.Visual
		id, ok := visual["id"].(string)
		req.True(ok && id != "", "path %d has no visual id", index)
		req.False(pathIDs[id], "duplicate path visual id %q", id)
		pathIDs[id] = true
		style, ok := visual["style"].(string)
		req.True(ok, "path %d has no edge style", index)
		req.Contains(style, "exitX=")
		req.Contains(style, "exitY=")
		req.Contains(style, "entryX=")
		req.Contains(style, "entryY=")
	}
}

func rectanglesOverlap(left, right []float64) bool {
	return left[0] < right[0]+right[2] &&
		right[0] < left[0]+left[2] &&
		left[1] < right[1]+right[3] &&
		right[1] < left[1]+left[3]
}

func TestWorkflowEdgeLabelIsValidUTF8(t *testing.T) {
	require.Equal(t, "Condición", workflowEdgeLabel(strings.Repeat("x", 23)))
}
