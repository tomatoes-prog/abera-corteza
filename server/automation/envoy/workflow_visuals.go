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
	"fmt"
	"sort"
	"strconv"

	"github.com/cortezaproject/corteza/server/automation/types"
	"github.com/cortezaproject/corteza/server/store"
	"go.uber.org/zap"
)

const (
	workflowVisualRootID          = "1"
	workflowVisualNodeW           = 200
	workflowVisualNodeH           = 80
	workflowVisualX               = 320
	workflowVisualY               = 80
	workflowVisualXGap            = 320
	workflowVisualYGap            = 180
	workflowVisualPreviousNodeW   = 240
	workflowVisualPreviousNodeH   = 84
	workflowVisualLegacyEdgeStyle = "edgeStyle=orthogonalEdgeStyle;rounded=0;orthogonalLoop=1;jettySize=auto;html=1;"
	workflowVisualLayoutID        = "abera-workflow-layout-v3"
	workflowVisualLayoutV1        = "abera-workflow-layout-v1"
	workflowVisualLayoutV2        = "abera-workflow-layout-v2"
	workflowVisualEdgeStyleID     = "abera-workflow-edge-layout-v3"
)

type workflowEdgeAnchor struct {
	sourceY float64
	targetY float64
}

// ensureWorkflowVisuals adds the mxGraph metadata used by the workflow editor
// while preserving any layout that was explicitly saved by a user.
//
// Workflow execution only needs steps and paths. The editor additionally needs
// coordinates and graph identifiers for every vertex and edge, so templates
// that only describe the executable graph must be completed before they are
// persisted.
func ensureWorkflowVisuals(wf *types.Workflow) bool {
	if wf == nil {
		return false
	}

	positions := workflowVisualPositions(wf)
	anchors := workflowEdgeAnchors(wf)
	reflow := workflowNeedsReflow(wf)
	changed := false

	for index, step := range wf.Steps {
		if step == nil {
			continue
		}
		if !reflow && hasVisualFields(step.Meta.Visual, "id", "xywh", "parent") {
			if repairWorkflowNodeVisual(step.Meta.Visual) {
				changed = true
			}
			continue
		}

		position := positions[step.ID]
		if position == nil {
			position = []float64{workflowVisualX, workflowVisualY + float64(index*workflowVisualYGap)}
		}

		step.Meta.Visual = map[string]interface{}{
			"id":     strconv.FormatUint(step.ID, 10),
			"value":  workflowStepLabel(step),
			"xywh":   []float64{position[0], position[1], workflowVisualNodeW, workflowVisualNodeH},
			"parent": workflowVisualRootID,
			"layout": workflowVisualLayoutID,
		}
		changed = true
	}

	for index, path := range wf.Paths {
		if path == nil {
			continue
		}

		anchor := anchors[index]
		if reflow || !hasVisualFields(path.Meta.Visual, "id", "parent") {
			pathID := visualID(path.Meta.Visual, fmt.Sprintf("path-%d-%d-%d", path.ParentID, path.ChildID, index))
			path.Meta.Visual = workflowEdgeVisual(
				pathID,
				workflowVisualRootID,
				workflowEdgeLabel(path.Expr),
				anchor.sourceY,
				anchor.targetY,
			)
			changed = true
		} else if repairWorkflowEdgeVisual(path.Meta.Visual, workflowEdgeLabel(path.Expr), anchor.sourceY, anchor.targetY) {
			changed = true
		}
	}

	return changed
}

// repairWorkflowVisuals repairs workflows already stored in an instance. This
// is intentionally separate from the Envoy defaults because existing
// installations use OnConflictSkip during provisioning.
func RepairWorkflowVisuals(ctx context.Context, log *zap.Logger, s store.Storer) error {
	workflows, _, err := store.SearchAutomationWorkflows(ctx, s, types.WorkflowFilter{})
	if err != nil {
		return fmt.Errorf("failed to search workflows for visual repair: %w", err)
	}

	triggers, _, err := store.SearchAutomationTriggers(ctx, s, types.TriggerFilter{})
	if err != nil {
		return fmt.Errorf("failed to search workflow triggers for visual repair: %w", err)
	}

	triggersByWorkflow := make(map[uint64][]*types.Trigger)
	for _, trigger := range triggers {
		if trigger != nil {
			triggersByWorkflow[trigger.WorkflowID] = append(triggersByWorkflow[trigger.WorkflowID], trigger)
		}
	}

	for _, workflow := range workflows {
		if workflow == nil {
			continue
		}

		workflowChanged := ensureWorkflowVisuals(workflow)
		changedTriggers := make([]*types.Trigger, 0)
		positions := workflowVisualPositions(workflow)
		workflowTriggers := triggersByWorkflow[workflow.ID]
		sort.Slice(workflowTriggers, func(i, j int) bool { return workflowTriggers[i].ID < workflowTriggers[j].ID })
		triggersByStep := make(map[uint64][]*types.Trigger)
		for _, trigger := range workflowTriggers {
			triggersByStep[trigger.StepID] = append(triggersByStep[trigger.StepID], trigger)
		}
		for _, trigger := range workflowTriggers {
			stepTriggers := triggersByStep[trigger.StepID]
			triggerIndex := 0
			for index, candidate := range stepTriggers {
				if candidate.ID == trigger.ID {
					triggerIndex = index
					break
				}
			}
			if ensureTriggerVisual(trigger, positions, triggerIndex, len(stepTriggers)) {
				changedTriggers = append(changedTriggers, trigger)
			}
		}

		if !workflowChanged && len(changedTriggers) == 0 {
			continue
		}

		if workflowChanged {
			if err = store.UpdateAutomationWorkflow(ctx, s, workflow); err != nil {
				return fmt.Errorf("failed to persist visual layout for workflow %q: %w", workflow.Handle, err)
			}
		}

		for _, trigger := range changedTriggers {
			if err = store.UpdateAutomationTrigger(ctx, s, trigger); err != nil {
				return fmt.Errorf("failed to persist visual layout for workflow trigger %d: %w", trigger.ID, err)
			}
		}

		log.Debug("workflow visual layout repaired", zap.String("handle", workflow.Handle))
	}

	return nil
}

func ensureTriggerVisual(trigger *types.Trigger, positions map[uint64][]float64, index, count int) bool {
	if trigger == nil {
		return false
	}

	if trigger.Meta != nil && hasVisualFields(trigger.Meta.Visual, "id", "xywh", "parent", "edges") && !legacyTriggerVisual(trigger.Meta.Visual) {
		if !generatedWorkflowVisual(trigger.Meta.Visual) {
			changed := repairWorkflowNodeVisual(trigger.Meta.Visual)
			if repairTriggerEdgeVisuals(trigger.Meta.Visual, workflowAnchor(index, count)) {
				changed = true
			}
			return changed
		}
	}

	if trigger.Meta == nil {
		trigger.Meta = &types.TriggerMeta{}
	}

	position := positions[trigger.StepID]
	if position == nil {
		position = []float64{workflowVisualX, workflowVisualY}
	}
	position = []float64{position[0], position[1] + triggerOffset(index, count)}

	visualID := visualID(triggerVisual(trigger), fmt.Sprintf("trigger-%d", trigger.ID))
	edges := []interface{}{}
	if trigger.StepID > 0 {
		targetID := strconv.FormatUint(trigger.StepID, 10)
		edgeID := triggerEdgeID(triggerVisual(trigger), fmt.Sprintf("trigger-edge-%d", trigger.ID))
		edges = append(edges, map[string]interface{}{
			"parentID": visualID,
			"childID":  targetID,
			"meta": map[string]interface{}{
				"label":       "",
				"description": "",
				"visual": workflowEdgeVisual(
					edgeID,
					workflowVisualRootID,
					"",
					0.5,
					workflowAnchor(index, count),
				),
			},
		})
	}

	trigger.Meta.Visual = map[string]interface{}{
		"id":          visualID,
		"value":       fmt.Sprintf("%s: %s", trigger.ResourceType, trigger.EventType),
		"defaultName": false,
		"xywh":        []float64{position[0] - workflowVisualXGap, position[1], workflowVisualNodeW, workflowVisualNodeH},
		"parent":      workflowVisualRootID,
		"edges":       edges,
		"layout":      workflowVisualLayoutID,
	}

	return true
}

func hasTriggerVisual(meta *types.TriggerMeta) bool {
	return meta != nil && hasVisualFields(meta.Visual, "id", "xywh", "parent", "edges")
}

func hasVisualFields(visual map[string]interface{}, fields ...string) bool {
	if visual == nil {
		return false
	}

	for _, field := range fields {
		value, ok := visual[field]
		if !ok || value == nil {
			return false
		}
	}

	return true
}

func visualID(visual map[string]interface{}, fallback string) string {
	if visual != nil {
		if id, ok := visual["id"].(string); ok && id != "" {
			return id
		}
	}
	return fallback
}

func triggerVisual(trigger *types.Trigger) map[string]interface{} {
	if trigger == nil || trigger.Meta == nil {
		return nil
	}
	return trigger.Meta.Visual
}

func triggerEdgeID(visual map[string]interface{}, fallback string) string {
	if visual == nil {
		return fallback
	}
	edges, ok := visual["edges"].([]interface{})
	if !ok || len(edges) == 0 {
		return fallback
	}
	edge, ok := edges[0].(map[string]interface{})
	if !ok {
		return fallback
	}
	meta, ok := edge["meta"].(map[string]interface{})
	if !ok {
		return fallback
	}
	edgeVisual, ok := meta["visual"].(map[string]interface{})
	if !ok {
		return fallback
	}
	return visualID(edgeVisual, fallback)
}

func workflowAnchor(index, count int) float64 {
	if count <= 1 {
		return 0.5
	}
	const margin = 0.18
	return margin + float64(index)*(1-2*margin)/float64(count-1)
}

func triggerOffset(index, count int) float64 {
	if count <= 1 {
		return 0
	}
	return (float64(index) - float64(count-1)/2) * (workflowVisualNodeH + 40)
}

func workflowEdgeStyle(sourceY, targetY float64) string {
	return fmt.Sprintf(
		"edgeStyle=orthogonalEdgeStyle;rounded=0;orthogonalLoop=1;jettySize=auto;sourceJettySize=0;targetJettySize=0;sourcePerimeterSpacing=0;targetPerimeterSpacing=0;perimeterSpacing=0;exitX=1;exitY=%.3f;exitDx=0;exitDy=0;entryX=0;entryY=%.3f;entryDx=0;entryDy=0;html=1;",
		sourceY,
		targetY,
	)
}

func workflowEdgeVisual(id, parent, value string, sourceY, targetY float64) map[string]interface{} {
	return map[string]interface{}{
		"id":          id,
		"value":       value,
		"parent":      parent,
		"points":      []interface{}{},
		"style":       workflowEdgeStyle(sourceY, targetY),
		"edgeStyleID": workflowVisualEdgeStyleID,
		"layout":      workflowVisualLayoutID,
	}
}

// repairWorkflowEdgeVisual updates only the connection style. In particular,
// it keeps points and identifiers written by a user while removing the
// default mxGraph jetty spacing that makes arrowheads appear detached from
// their target boxes.
func repairWorkflowEdgeVisual(visual map[string]interface{}, value string, sourceY, targetY float64) bool {
	if visual == nil {
		return false
	}

	changed := false
	style := workflowEdgeStyle(sourceY, targetY)
	if visual["style"] != style {
		visual["style"] = style
		changed = true
	}
	if visual["edgeStyleID"] != workflowVisualEdgeStyleID {
		visual["edgeStyleID"] = workflowVisualEdgeStyleID
		changed = true
	}
	if value != "" && visual["value"] != value {
		visual["value"] = value
		changed = true
	}

	return changed
}

func repairWorkflowNodeVisual(visual map[string]interface{}) bool {
	if visual == nil {
		return false
	}

	xywh, ok := visualXYWH(visual)
	if !ok || xywh[2] != workflowVisualPreviousNodeW || xywh[3] != workflowVisualPreviousNodeH {
		return false
	}

	visual["xywh"] = []float64{xywh[0], xywh[1], workflowVisualNodeW, workflowVisualNodeH}
	return true
}

func repairTriggerEdgeVisuals(visual map[string]interface{}, targetY float64) bool {
	edges, ok := visual["edges"].([]interface{})
	if !ok {
		return false
	}

	changed := false
	for _, rawEdge := range edges {
		edge, ok := rawEdge.(map[string]interface{})
		if !ok {
			continue
		}
		meta, ok := edge["meta"].(map[string]interface{})
		if !ok {
			continue
		}
		edgeVisual, ok := meta["visual"].(map[string]interface{})
		if !ok {
			continue
		}
		if repairWorkflowEdgeVisual(edgeVisual, "", 0.5, targetY) {
			changed = true
		}
	}

	return changed
}

func workflowStepLabel(step *types.WorkflowStep) string {
	if step == nil {
		return ""
	}
	if step.Meta.Name != "" {
		return step.Meta.Name
	}
	if step.Ref != "" {
		return step.Ref
	}
	return string(step.Kind)
}

// workflowEdgeLabel keeps long expressions from covering adjacent nodes. The
// complete expression remains in the path configuration and is available when
// the edge is selected in the editor.
func workflowEdgeLabel(expr string) string {
	if expr == "" {
		return ""
	}
	if len([]rune(expr)) <= 22 {
		return expr
	}
	return "Condición"
}

// workflowNeedsReflow identifies layouts generated by the previous compact
// grid. User layouts are kept unless they match that legacy signature.
func workflowNeedsReflow(wf *types.Workflow) bool {
	if wf == nil || len(wf.Steps) == 0 {
		return false
	}

	hasVisual := false
	missingVisual := false
	legacy := true
	for _, step := range wf.Steps {
		if step == nil || !hasVisualFields(step.Meta.Visual, "id", "xywh", "parent") {
			missingVisual = true
			continue
		}
		hasVisual = true
		if step.Meta.Visual["layout"] == workflowVisualLayoutID {
			legacy = false
			continue
		}
		if generatedWorkflowVisual(step.Meta.Visual) {
			return true
		}
		if !legacyNodeVisual(step.Meta.Visual) {
			return false
		}
	}

	for _, path := range wf.Paths {
		if path == nil || !hasVisualFields(path.Meta.Visual, "id", "parent") {
			missingVisual = true
			continue
		}
		if path.Meta.Visual["layout"] == workflowVisualLayoutID {
			legacy = false
			continue
		}
		if generatedWorkflowVisual(path.Meta.Visual) {
			return true
		}
		if !legacyEdgeVisual(path.Meta.Visual) {
			return false
		}
	}

	return hasVisual && !missingVisual && legacy
}

func generatedWorkflowVisual(visual map[string]interface{}) bool {
	if visual == nil {
		return false
	}

	switch visual["layout"] {
	case workflowVisualLayoutV1, workflowVisualLayoutV2:
		return true
	default:
		return false
	}
}

func legacyNodeVisual(visual map[string]interface{}) bool {
	xywh, ok := visualXYWH(visual)
	return ok && visual["layout"] == nil && visual["parent"] == workflowVisualRootID && xywh[2] == 220 && xywh[3] == 72
}

func legacyEdgeVisual(visual map[string]interface{}) bool {
	if visual["layout"] != nil || visual["parent"] != workflowVisualRootID || visual["style"] != workflowVisualLegacyEdgeStyle {
		return false
	}

	points, ok := visual["points"].([]interface{})
	return ok && len(points) == 0
}

func legacyTriggerVisual(visual map[string]interface{}) bool {
	if visual["layout"] != nil || !legacyNodeVisual(visual) {
		return false
	}

	edges, ok := visual["edges"].([]interface{})
	if !ok || len(edges) != 1 {
		return false
	}

	edge, ok := edges[0].(map[string]interface{})
	if !ok {
		return false
	}
	meta, ok := edge["meta"].(map[string]interface{})
	if !ok {
		return false
	}
	edgeVisual, ok := meta["visual"].(map[string]interface{})
	return ok && legacyEdgeVisual(edgeVisual)
}

func visualXYWH(visual map[string]interface{}) ([]float64, bool) {
	value, ok := visual["xywh"]
	if !ok {
		return nil, false
	}

	switch values := value.(type) {
	case []float64:
		return values, len(values) == 4
	case []interface{}:
		out := make([]float64, len(values))
		for i, value := range values {
			switch number := value.(type) {
			case float64:
				out[i] = number
			case int:
				out[i] = float64(number)
			default:
				return nil, false
			}
		}
		return out, len(out) == 4
	default:
		return nil, false
	}
}

func workflowVisualPositions(wf *types.Workflow) map[uint64][]float64 {
	positions := make(map[uint64][]float64)
	if wf == nil {
		return positions
	}

	stepIDs := make([]uint64, 0, len(wf.Steps))
	known := make(map[uint64]bool, len(wf.Steps))
	indegree := make(map[uint64]int, len(wf.Steps))
	children := make(map[uint64][]uint64, len(wf.Steps))
	parents := make(map[uint64][]uint64, len(wf.Steps))
	for _, step := range wf.Steps {
		if step == nil || known[step.ID] {
			continue
		}
		known[step.ID] = true
		stepIDs = append(stepIDs, step.ID)
		indegree[step.ID] = 0
	}

	for _, path := range wf.Paths {
		if path == nil || !known[path.ParentID] || !known[path.ChildID] {
			continue
		}
		children[path.ParentID] = append(children[path.ParentID], path.ChildID)
		parents[path.ChildID] = append(parents[path.ChildID], path.ParentID)
		indegree[path.ChildID]++
	}

	for parent := range children {
		sort.Slice(children[parent], func(i, j int) bool { return children[parent][i] < children[parent][j] })
	}
	sort.Slice(stepIDs, func(i, j int) bool { return stepIDs[i] < stepIDs[j] })

	rank := make(map[uint64]int, len(stepIDs))
	queue := make([]uint64, 0, len(stepIDs))
	for _, id := range stepIDs {
		if indegree[id] == 0 {
			rank[id] = 0
			queue = append(queue, id)
		}
	}

	maxRank := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if rank[id] > maxRank {
			maxRank = rank[id]
		}

		for _, child := range children[id] {
			if rank[child] < rank[id]+1 {
				rank[child] = rank[id] + 1
			}
			indegree[child]--
			if indegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	// Cyclic or disconnected definitions still need a stable place on the
	// canvas. Put anything not reached by the DAG walk after the normal graph.
	for _, id := range stepIDs {
		if _, ok := rank[id]; !ok {
			maxRank++
			rank[id] = maxRank
		}
	}

	byRank := make(map[int][]uint64)
	for _, id := range stepIDs {
		byRank[rank[id]] = append(byRank[rank[id]], id)
	}
	for _, ids := range byRank {
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	}
	workflowOrderRanks(byRank, parents, children, maxRank)
	for currentRank, ids := range byRank {
		for index, id := range ids {
			positions[id] = []float64{
				workflowVisualX + float64(currentRank*workflowVisualXGap),
				workflowVisualY + float64(index*workflowVisualYGap),
			}
		}
	}

	return positions
}

// workflowOrderRanks applies deterministic barycenter sweeps to the layered
// layout. Dependencies remain horizontal while gateway branches and joins are
// kept in a consistent vertical order to reduce crossings.
func workflowOrderRanks(byRank map[int][]uint64, parents, children map[uint64][]uint64, maxRank int) {
	order := make(map[uint64]int)
	refreshWorkflowOrder(byRank, order)

	for pass := 0; pass < 4; pass++ {
		for rank := 1; rank <= maxRank; rank++ {
			reorderWorkflowRank(byRank[rank], parents, order)
			refreshWorkflowOrder(byRank, order)
		}
		for rank := maxRank - 1; rank >= 0; rank-- {
			reorderWorkflowRank(byRank[rank], children, order)
			refreshWorkflowOrder(byRank, order)
		}
	}
}

func reorderWorkflowRank(ids []uint64, neighbours map[uint64][]uint64, order map[uint64]int) {
	if len(ids) < 2 {
		return
	}

	sort.SliceStable(ids, func(i, j int) bool {
		left, leftHasNeighbours := workflowBarycenter(ids[i], neighbours, order)
		right, rightHasNeighbours := workflowBarycenter(ids[j], neighbours, order)
		if leftHasNeighbours != rightHasNeighbours {
			return leftHasNeighbours
		}
		if left != right {
			return left < right
		}
		if order[ids[i]] != order[ids[j]] {
			return order[ids[i]] < order[ids[j]]
		}
		return ids[i] < ids[j]
	})
}

func workflowBarycenter(id uint64, neighbours map[uint64][]uint64, order map[uint64]int) (float64, bool) {
	ids := neighbours[id]
	if len(ids) == 0 {
		return 0, false
	}

	var total float64
	count := 0
	for _, neighbour := range ids {
		if position, ok := order[neighbour]; ok {
			total += float64(position)
			count++
		}
	}
	if count == 0 {
		return 0, false
	}
	return total / float64(count), true
}

func refreshWorkflowOrder(byRank map[int][]uint64, order map[uint64]int) {
	for _, ids := range byRank {
		for index, id := range ids {
			order[id] = index
		}
	}
}

func workflowEdgeAnchors(wf *types.Workflow) map[int]workflowEdgeAnchor {
	anchors := make(map[int]workflowEdgeAnchor)
	if wf == nil {
		return anchors
	}

	outgoing := make(map[uint64][]int)
	incoming := make(map[uint64][]int)
	for index, path := range wf.Paths {
		if path == nil {
			continue
		}
		outgoing[path.ParentID] = append(outgoing[path.ParentID], index)
		incoming[path.ChildID] = append(incoming[path.ChildID], index)
		anchors[index] = workflowEdgeAnchor{sourceY: 0.5, targetY: 0.5}
	}

	for _, indexes := range outgoing {
		sort.Slice(indexes, func(i, j int) bool {
			left, right := wf.Paths[indexes[i]], wf.Paths[indexes[j]]
			if left.ChildID != right.ChildID {
				return left.ChildID < right.ChildID
			}
			return indexes[i] < indexes[j]
		})
		for index, pathIndex := range indexes {
			anchor := anchors[pathIndex]
			anchor.sourceY = workflowAnchor(index, len(indexes))
			anchors[pathIndex] = anchor
		}
	}

	for _, indexes := range incoming {
		sort.Slice(indexes, func(i, j int) bool {
			left, right := wf.Paths[indexes[i]], wf.Paths[indexes[j]]
			if left.ParentID != right.ParentID {
				return left.ParentID < right.ParentID
			}
			return indexes[i] < indexes[j]
		})
		for index, pathIndex := range indexes {
			anchor := anchors[pathIndex]
			anchor.targetY = workflowAnchor(index, len(indexes))
			anchors[pathIndex] = anchor
		}
	}

	return anchors
}
