// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func readYAMLDocument(t *testing.T, path string) *yaml.Node {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc yaml.Node
	if err = yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return documentMap(&doc)
}

func TestV2ManifestsModulesAndDescriptions(t *testing.T) {
	root := repositoryRoot(t)
	for _, templateID := range templateIDs {
		t.Run(templateID, func(t *testing.T) {
			dir := filepath.Join(root, "templates", templateID)
			manifest := readYAMLDocument(t, filepath.Join(dir, "manifest.yaml"))
			if got := scalarValue(mapValue(manifest, "version")); got != "2.0.0" {
				t.Fatalf("manifest version = %q, want 2.0.0", got)
			}
			if got := scalarValue(mapValue(manifest, "license")); got != "Apache-2.0" {
				t.Fatalf("manifest license = %q, want Apache-2.0", got)
			}
			if got := scalarValue(mapValue(manifest, "defaultLocale")); got != "es" {
				t.Fatalf("manifest locale = %q, want es", got)
			}

			modules := mapValue(readYAMLDocument(t, filepath.Join(dir, "provision", "modules.yaml")), "modules")
			locale := mapValue(readYAMLDocument(t, filepath.Join(dir, "provision", "descriptions-es.yaml")), "locale")
			translations := mapValue(locale, "es")
			if modules == nil || translations == nil {
				t.Fatal("modules or Spanish descriptions mapping missing")
			}

			moduleHandles := map[string]bool{}
			for i := 0; i+1 < len(modules.Content); i += 2 {
				moduleHandles[modules.Content[i].Value] = true
			}
			for i := 0; i+1 < len(modules.Content); i += 2 {
				moduleHandle, module := modules.Content[i].Value, modules.Content[i+1]
				moduleKey := fmt.Sprintf("corteza::compose:module/%s/%s", templateID, moduleHandle)
				moduleTranslation := mapValue(translations, moduleKey)
				translatedModuleDescription := strings.TrimSpace(scalarValue(mapValue(moduleTranslation, "meta.description")))
				if len(translatedModuleDescription) < 60 {
					t.Errorf("module %s needs a complete Spanish description", moduleHandle)
				}
				embeddedModuleDescription := strings.TrimSpace(scalarValue(mapValue(mapValue(module, "meta"), "description")))
				if embeddedModuleDescription != translatedModuleDescription {
					t.Errorf("module %s description is not embedded in modules.yaml", moduleHandle)
				}
				fields := mapValue(module, "fields")
				for j := 0; fields != nil && j+1 < len(fields.Content); j += 2 {
					fieldHandle, field := fields.Content[j].Value, fields.Content[j+1]
					fieldKey := fmt.Sprintf("corteza::compose:module-field/%s/%s/%s", templateID, moduleHandle, fieldHandle)
					fieldTranslation := mapValue(translations, fieldKey)
					fieldDescription := mapValue(mapValue(field, "options"), "description")
					for translatedKey, embeddedKey := range map[string]string{"meta.description.view": "view", "meta.description.edit": "edit"} {
						translated := strings.TrimSpace(scalarValue(mapValue(fieldTranslation, translatedKey)))
						if len(translated) < 35 {
							t.Errorf("field %s.%s lacks %s", moduleHandle, fieldHandle, translatedKey)
						}
						if embedded := strings.TrimSpace(scalarValue(mapValue(fieldDescription, embeddedKey))); embedded != translated {
							t.Errorf("field %s.%s does not embed %s", moduleHandle, fieldHandle, embeddedKey)
						}
					}
					if scalarValue(mapValue(field, "kind")) == "Record" {
						target := scalarValue(mapValue(mapValue(field, "options"), "module"))
						if !moduleHandles[target] {
							t.Errorf("field %s.%s points to missing module %q", moduleHandle, fieldHandle, target)
						}
					}
				}
			}
		})
	}
}

type pageStats struct {
	metrics, maps, organizers, recordLists int
	queueFound                             bool
}

func TestV2PageLayoutsAndDistinctDesigns(t *testing.T) {
	root := repositoryRoot(t)
	hashes := map[[32]byte]string{}
	expectedMaps := map[string]int{"inmobiliaria-co": 1, "servicios-tecnicos-co": 1}
	expectedQueues := map[string]bool{"centro-contacto-co": true, "soporte-renovaciones-co": true}

	for _, templateID := range templateIDs {
		t.Run(templateID, func(t *testing.T) {
			path := filepath.Join(root, "templates", templateID, "provision", "pages.yaml")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(raw)
			if previous, duplicate := hashes[hash]; duplicate {
				t.Fatalf("pages are byte-identical to %s; templates must retain distinct designs", previous)
			}
			hashes[hash] = templateID

			pages := mapValue(readYAMLDocument(t, path), "pages")
			stats := pageStats{}
			validatePageMap(t, pages, &stats)
			if stats.metrics != 4 {
				t.Errorf("metric blocks = %d, want 4", stats.metrics)
			}
			if stats.maps != expectedMaps[templateID] {
				t.Errorf("map blocks = %d, want %d", stats.maps, expectedMaps[templateID])
			}
			if stats.organizers < 1 {
				t.Error("pipeline RecordOrganizer missing")
			}
			if stats.recordLists < 5 {
				t.Errorf("only %d operational lists found", stats.recordLists)
			}
			if stats.queueFound != expectedQueues[templateID] {
				t.Errorf("personal queue present = %v, want %v", stats.queueFound, expectedQueues[templateID])
			}
		})
	}
}

func TestV2HomeBlocksAndMapConfiguration(t *testing.T) {
	root := repositoryRoot(t)
	for _, templateID := range templateIDs {
		t.Run(templateID, func(t *testing.T) {
			pages := mapValue(readYAMLDocument(t, filepath.Join(root, "templates", templateID, "provision", "pages.yaml")), "pages")
			home := findPage(pages, "inicio")
			if home == nil {
				t.Fatal("home page missing")
			}
			metricCount, operationalLists := 0, 0
			for _, block := range sequenceContent(mapValue(home, "blocks")) {
				kind := scalarValue(mapValue(block, "kind"))
				xywh := mapValue(block, "xywh")
				switch kind {
				case "Metric":
					metricCount++
					if strings.HasPrefix(scalarValue(mapValue(block, "title")), "Indicador ·") {
						t.Error("metric title retains redundant prefix")
					}
					if strings.Contains(scalarValue(mapValue(block, "description")), "Indicador accionable calculado") {
						t.Error("metric retains generic long description")
					}
					if xywh == nil || intValue(xywh.Content[3]) < homeMetricHeight {
						t.Errorf("metric height below %d", homeMetricHeight)
					}
				case "RecordList":
					operationalLists++
					if xywh == nil || intValue(xywh.Content[3]) < homeOperationalListHeight {
						t.Errorf("home operational list height below %d", homeOperationalListHeight)
					}
				}
			}
			if metricCount != 4 {
				t.Errorf("home metric count = %d, want 4", metricCount)
			}
			if operationalLists == 0 {
				t.Error("home page has no operational lists")
			}

			spec := visualConfigs[templateID].mapBlock
			if spec.page == "" {
				return
			}
			page := findPage(pages, spec.page)
			block := blockByTitleAndKind(page, spec.title, "Geometry")
			if block == nil {
				t.Fatalf("Geometry block %q missing", spec.title)
			}
			options := mapValue(block, "options")
			expectedZoom := spec.zoomStarting
			if expectedZoom == 0 {
				expectedZoom = 6
			}
			if intValue(mapValue(options, "zoomStarting")) != expectedZoom {
				t.Errorf("unexpected starting zoom")
			}
			feeds := sequenceContent(mapValue(options, "feeds"))
			if len(feeds) != 1 {
				t.Fatalf("Geometry feed count = %d, want 1", len(feeds))
			}
			feed := feeds[0]
			feedOptions := mapValue(feed, "options")
			if got := scalarValue(mapValue(feedOptions, "module")); got != spec.module {
				t.Errorf("Geometry module = %q, want %q", got, spec.module)
			}
			if got := scalarValue(mapValue(feed, "titleField")); got != spec.titleField {
				t.Errorf("Geometry title field = %q, want %q", got, spec.titleField)
			}
			if got := scalarValue(mapValue(feed, "geometryField")); got != spec.geometryField {
				t.Errorf("Geometry field = %q, want %q", got, spec.geometryField)
			}
			if got := scalarValue(mapValue(feedOptions, "prefilter")); got != spec.filter {
				t.Errorf("Geometry prefilter = %q, want %q", got, spec.filter)
			}
			if len(spec.bounds) > 0 && mapValue(options, "bounds") == nil {
				t.Error("Geometry bounds missing")
			}
		})
	}
}

func validatePageMap(t *testing.T, pages *yaml.Node, stats *pageStats) {
	t.Helper()
	if pages == nil || pages.Kind != yaml.MappingNode {
		t.Fatal("pages mapping missing")
	}
	for i := 0; i+1 < len(pages.Content); i += 2 {
		handle, page := pages.Content[i].Value, pages.Content[i+1]
		if handle == "mi-cola" {
			stats.queueFound = true
		}
		if strings.TrimSpace(scalarValue(mapValue(page, "description"))) == "" {
			t.Errorf("page %s has no description", handle)
		}
		blocks := mapValue(page, "blocks")
		if blocks != nil {
			rects := make([][4]int, 0, len(blocks.Content))
			for index, block := range blocks.Content {
				kind := scalarValue(mapValue(block, "kind"))
				switch kind {
				case "Metric":
					stats.metrics++
				case "Geometry":
					stats.maps++
				case "RecordOrganizer":
					stats.organizers++
				case "RecordList":
					stats.recordLists++
					options := mapValue(block, "options")
					for _, option := range []string{"allowExport", "showTotalCount", "showRecordPerPageOption", "inlineValueFiltering"} {
						if !boolValue(mapValue(options, option)) {
							t.Errorf("page %s RecordList %d does not enable %s", handle, index, option)
						}
					}
				}
				if strings.TrimSpace(scalarValue(mapValue(block, "description"))) == "" {
					t.Errorf("page %s block %d (%s) has no description", handle, index, kind)
				}
				xywh := mapValue(block, "xywh")
				if xywh == nil || len(xywh.Content) != 4 {
					t.Errorf("page %s block %d has invalid xywh", handle, index)
					continue
				}
				rect := [4]int{intValue(xywh.Content[0]), intValue(xywh.Content[1]), intValue(xywh.Content[2]), intValue(xywh.Content[3])}
				if rect[0] < 0 || rect[1] < 0 || rect[2] <= 0 || rect[0]+rect[2] > 48 {
					t.Errorf("page %s block %d outside 48-column grid: %v", handle, index, rect)
				}
				if rect[3] < minimumHeightForBlock(block, kind) {
					t.Errorf("page %s block %d height %d below minimum %d", handle, index, rect[3], minimumHeightForBlock(block, kind))
				}
				for previous, other := range rects {
					if rectanglesOverlap(rect, other) {
						t.Errorf("page %s blocks %d and %d overlap: %v / %v", handle, previous, index, other, rect)
					}
				}
				rects = append(rects, rect)
			}
		}
		if children := mapValue(page, "children"); children != nil {
			validatePageMap(t, children, stats)
		}
		if children := mapValue(page, "pages"); children != nil {
			validatePageMap(t, children, stats)
		}
	}
}

func rectanglesOverlap(a, b [4]int) bool {
	return a[0] < b[0]+b[2] && b[0] < a[0]+a[2] && a[1] < b[1]+b[3] && b[1] < a[1]+a[3]
}

func TestV2ReportsAndWorkflows(t *testing.T) {
	root := repositoryRoot(t)
	for _, templateID := range templateIDs {
		t.Run(templateID, func(t *testing.T) {
			provision := filepath.Join(root, "templates", templateID, "provision")
			reports := mapValue(readYAMLDocument(t, filepath.Join(provision, "reports.yaml")), "report")
			if reports == nil || len(reports.Content)/2 != 2 {
				t.Fatalf("report count = %d, want 2", len(reports.Content)/2)
			}
			for i := 0; i+1 < len(reports.Content); i += 2 {
				report := reports.Content[i+1]
				meta := mapValue(report, "meta")
				if strings.TrimSpace(scalarValue(mapValue(meta, "description"))) == "" {
					t.Errorf("report %s has no description", reports.Content[i].Value)
				}
			}

			workflowFiles, err := filepath.Glob(filepath.Join(provision, "workflows*.yaml"))
			if err != nil || len(workflowFiles) < 2 {
				t.Fatalf("workflow files: %v, %v", workflowFiles, err)
			}
			examples := 0
			for _, path := range workflowFiles {
				workflows := mapValue(readYAMLDocument(t, path), "workflows")
				for i := 0; workflows != nil && i+1 < len(workflows.Content); i += 2 {
					handle, workflow := workflows.Content[i].Value, workflows.Content[i+1]
					meta := mapValue(workflow, "meta")
					if strings.TrimSpace(scalarValue(mapValue(meta, "description"))) == "" {
						t.Errorf("workflow %s has no functional description", handle)
					}
					validateWorkflowGraph(t, handle, workflow)
					if strings.HasPrefix(handle, "ejemplo-") {
						examples++
						if boolValue(mapValue(workflow, "enabled")) {
							t.Errorf("external example %s is enabled", handle)
						}
						for _, trigger := range sequenceContent(mapValue(workflow, "triggers")) {
							if boolValue(mapValue(trigger, "enabled")) {
								t.Errorf("external example %s has an enabled trigger", handle)
							}
						}
					}
				}
			}
			if examples != 2 {
				t.Errorf("external workflow examples = %d, want 2", examples)
			}
		})
	}
}

func validateWorkflowGraph(t *testing.T, handle string, workflow *yaml.Node) {
	t.Helper()
	steps := map[uint64]bool{}
	for _, step := range sequenceContent(mapValue(workflow, "steps")) {
		id := uint64(intValue(mapValue(step, "id")))
		if id == 0 || steps[id] {
			t.Errorf("workflow %s has missing or duplicate step ID %d", handle, id)
		}
		steps[id] = true
	}
	for _, trigger := range sequenceContent(mapValue(workflow, "triggers")) {
		stepID := uint64(intValue(mapValue(trigger, "stepID")))
		if !steps[stepID] {
			t.Errorf("workflow %s trigger points to missing step %d", handle, stepID)
		}
	}
	for _, path := range sequenceContent(mapValue(workflow, "paths")) {
		parent := uint64(intValue(mapValue(path, "parentid")))
		child := uint64(intValue(mapValue(path, "childid")))
		if !steps[parent] || !steps[child] {
			t.Errorf("workflow %s path %d -> %d is orphaned", handle, parent, child)
		}
		if parent == child {
			t.Errorf("workflow %s contains self-link on step %d", handle, parent)
		}
	}
}

func sequenceContent(node *yaml.Node) []*yaml.Node {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	return node.Content
}

func TestV2MaterializerIsIdempotentAndNotRuntimeWired(t *testing.T) {
	root := repositoryRoot(t)
	for _, runtimeFile := range []string{"Dockerfile", "docker-entrypoint.sh", "docker-template-select.sh"} {
		raw, err := os.ReadFile(filepath.Join(root, runtimeFile))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(bytes.ToLower(raw), []byte("template-v2")) {
			t.Fatalf("development materializer is wired into runtime file %s", runtimeFile)
		}
	}

	for _, templateID := range templateIDs {
		t.Run(templateID, func(t *testing.T) {
			source := filepath.Join(root, "templates", templateID, "provision")
			target := filepath.Join(t.TempDir(), templateID, "provision")
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"modules.yaml", "pages.yaml"} {
				raw, err := os.ReadFile(filepath.Join(source, name))
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(target, name), raw, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			run := func() map[string][32]byte {
				if err := normalizePages(templateID, filepath.Join(target, "pages.yaml")); err != nil {
					t.Fatal(err)
				}
				if err := generateDescriptions(templateID, filepath.Join(target, "modules.yaml"), filepath.Join(target, "descriptions-es.yaml")); err != nil {
					t.Fatal(err)
				}
				if err := generateReports(templateID, filepath.Join(target, "reports.yaml"), visualConfigs[templateID]); err != nil {
					t.Fatal(err)
				}
				if err := generateWorkflowExamples(filepath.Join(target, "workflows-v2.yaml"), workflowNoticeConfigs[templateID]); err != nil {
					t.Fatal(err)
				}
				out := map[string][32]byte{}
				for _, name := range []string{"pages.yaml", "descriptions-es.yaml", "reports.yaml", "workflows-v2.yaml"} {
					raw, err := os.ReadFile(filepath.Join(target, name))
					if err != nil {
						t.Fatal(err)
					}
					out[name] = sha256.Sum256(raw)
				}
				return out
			}
			first, second := run(), run()
			names := make([]string, 0, len(first))
			for name := range first {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				if first[name] != second[name] {
					t.Errorf("%s output changes on the second materialization", name)
				}
			}
		})
	}
}

func TestV2ResourcesContainNoMojibakeOrProviderCoupling(t *testing.T) {
	root := repositoryRoot(t)
	for _, templateID := range templateIDs {
		err := filepath.WalkDir(filepath.Join(root, "templates", templateID), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(raw)
			if strings.ContainsAny(text, "ÃÂ�") {
				t.Errorf("%s contains mojibake", path)
			}
			lower := strings.ToLower(text)
			for _, forbidden := range []string{"cloudformation", "secrets manager", "vault_addr", "amazon web services"} {
				if strings.Contains(lower, forbidden) {
					t.Errorf("%s couples templates to provider term %q", path, forbidden)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
