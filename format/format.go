package format

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/braydonk/yaml"
	"mvdan.cc/sh/v3/syntax"
)

func find(node *yaml.Node, name string) *yaml.Node {
	if node.Kind == yaml.DocumentNode {
		return find(node.Content[0], name)
	}
	for i := range node.Content {
		if node.Content[i].Value == name {
			if len(node.Content) >= i+1 && (node.Content[i+1].Kind == yaml.MappingNode || node.Content[i+1].Kind == yaml.SequenceNode) {
				return node.Content[i+1]
			}
		}
	}

	return nil
}

type pair struct {
	key   *yaml.Node
	value *yaml.Node
}

func toPairs(nodes *[]*yaml.Node) []pair {
	pairs := make([]pair, 0, len(*nodes)/2)
	for i := 0; i < len(*nodes)/2; i++ {
		pair := pair{(*nodes)[i*2], (*nodes)[i*2+1]}
		pairs = append(pairs, pair)
	}

	return pairs
}

func fromPairs(pairs []pair, nodes *[]*yaml.Node) {
	for i := 0; i < len(pairs); i++ {
		(*nodes)[i*2] = pairs[i].key
		(*nodes)[i*2+1] = pairs[i].value
	}
}

func rankedKeySort(nodes *[]*yaml.Node, rank map[string]int) {
	pairs := toPairs(nodes)
	sort.Slice(pairs, func(i, j int) bool {
		r1, ok := rank[pairs[i].key.Value]
		if !ok {
			r1 = 1000
		}
		r2, ok := rank[pairs[j].key.Value]
		if !ok {
			r2 = 1000
		}
		return r1 < r2
	})
	fromPairs(pairs, nodes)
}

func sortByName(node *yaml.Node, key string, rank map[string]int) {
	found := find(node, key)
	if found == nil {
		return
	}
	nodes := found.Content

	name := func(node *yaml.Node) string {
		pairs := toPairs(&node.Content)
		for _, pair := range pairs {
			if pair.key.Value == "name" {
				return pair.value.Value
			}
		}

		return ""
	}
	sort.Slice(nodes, func(i, j int) bool {
		name1 := name(nodes[i])
		name2 := name(nodes[j])

		return name1 < name2
	})
}

func Format(in io.Reader, out io.Writer) error {
	if closer, ok := in.(io.Closer); ok {
		defer closer.Close()
	}
	if closer, ok := out.(io.Closer); ok {
		defer closer.Close()
	}

	decoder := yaml.NewDecoder(in)

	parser := syntax.NewParser(syntax.KeepComments(true))
	printer := syntax.NewPrinter(syntax.Indent(2), syntax.KeepPadding(true))

	for {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("error: cannot decode object: %s", err)
		}

		rankedKeySort(&find(&node, "metadata").Content, map[string]int{
			"name":        1,
			"annotations": 2,
			"labels":      3,
		})

		spec := find(&node, "spec")
		rankedKeySort(&spec.Content, map[string]int{
			"displayName":  1,
			"description":  2,
			"params":       3,
			"results":      4,
			"volumes":      5,
			"workspaces":   6,
			"sidecars":     7,
			"stepTemplate": 8,
			"steps":        9,
		})

		sortByName(spec, "params", map[string]int{
			"displayName":  1,
			"description":  2,
			"params":       3,
			"results":      4,
			"volumes":      5,
			"workspaces":   6,
			"sidecars":     7,
			"stepTemplate": 8,
			"steps":        9,
		})
		sortByName(spec, "results", map[string]int{
			"name":        1,
			"description": 2,
			"value":       3,
			"type":        4,
			"properties":  5,
		})
		sortByName(spec, "workspaces", map[string]int{
			"name":        1,
			"description": 2,
			"mountPath":   3,
			"readOnly":    4,
			"optional":    5,
		})
		sortByName(spec, "sidecars", map[string]int{
			"name":                     1,
			"image":                    2,
			"command":                  3,
			"args":                     4,
			"workingDir":               5,
			"ports":                    6,
			"envFrom":                  7,
			"env":                      8,
			"computeResources":         9,
			"volumeMounts":             10,
			"volumeDevices":            11,
			"livenessProbe":            12,
			"readinessProbe":           13,
			"startupProbe":             14,
			"lifecycle":                15,
			"terminationMessagePath":   16,
			"terminationMessagePolicy": 17,
			"imagePullPolicy":          18,
			"securityContext":          19,
			"stdin":                    20,
			"stdinOnce":                21,
			"omitempty":                22,
			"script":                   23,
			"workspaces":               24,
		})

		steps := find(spec, "steps")

		for i := 0; i < len(steps.Content); i++ {
			step := steps.Content[i]
			rankedKeySort(&step.Content, map[string]int{
				"name":             1,
				"image":            2,
				"imagePullPolicy":  3,
				"command":          4,
				"args":             5,
				"params":           6,
				"results":          7,
				"workingDir":       8,
				"volumeMounts":     9,
				"volumeDevices":    10,
				"workspaces":       11,
				"envFrom":          12,
				"env":              13,
				"script":           14,
				"computeResources": 15,
				"securityContext":  16,
				"timeout":          17,
				"onError":          18,
				"stdoutConfig":     19,
				"stderrConfig":     20,
				"ref":              21,
			})

			for j := 0; j < len(step.Content); j += 2 {
				if step.Content[j].Value != "script" {
					continue
				}

				f, err := parser.Parse(strings.NewReader(step.Content[j+1].Value), "")
				if err != nil {
					break
				}

				sh := bytes.Buffer{}
				if err := printer.Print(&sh, f); err != nil {
					break
				}

				step.Content[j+1].Value = sh.String()
			}
		}

		encoder := yaml.NewEncoder(out)
		defer encoder.Close()

		encoder.SetLineBreakStyle(yaml.LineBreakStyleLF)
		encoder.SetExplicitDocumentStart(true)
		encoder.SetWidth(72)

		if err := encoder.Encode(&node); err != nil {
			return fmt.Errorf("error: cannot encode object: %s", err)
		}
	}

	return nil
}