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
	if node == nil {
		return nil
	}

	if node.Kind == yaml.DocumentNode {
		return find(node.Content[0], name)
	}

	for i := range node.Content {
		if node.Content[i].Value == name {
			if len(node.Content) >= i+1 {
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
		k1 := pairs[i].key.Value
		r1, ok := rank[k1]
		if !ok {
			r1 = 1000
		}
		k2 := pairs[j].key.Value
		r2, ok := rank[k2]
		if !ok {
			r2 = 1000
		}
		return r1 < r2 || (r1 == r2 && k1 < k2)
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

	for i := range nodes {
		rankedKeySort(&nodes[i].Content, rank)
	}
}

func deleteIf(node *yaml.Node, key string, cond func(*yaml.Node) bool) {
	n := find(node, key)
	if n == nil {
		return
	}

	if cond(n) {
		x := node.Content[:0]
		for i := 0; i < len(node.Content); i++ {
			if node.Content[i].Value != key {
				x = append(x, node.Content[i])
			} else {
				i++
			}
		}

		node.Content = x
	}
}

func unquote(node *yaml.Node) {
	switch node.Kind {
	case yaml.DocumentNode:
		unquote(node.Content[0])
	case yaml.MappingNode:
		for i := 1; i < len(node.Content); i += 2 {
			unquote(node.Content[i])
		}
	case yaml.SequenceNode:
		for i := 0; i < len(node.Content); i++ {
			unquote(node.Content[i])
		}
	case yaml.ScalarNode:
		node.Style = node.Style & ^yaml.DoubleQuotedStyle & ^yaml.SingleQuotedStyle & ^yaml.LiteralStyle & ^yaml.FoldedStyle & ^yaml.FlowStyle
	}
}

func sortEverything(node *yaml.Node) {
	switch node.Kind {
	case yaml.DocumentNode:
		sortEverything(node.Content[0])
	case yaml.MappingNode:
		rankedKeySort(&node.Content, map[string]int{})
		for i := 1; i < len(node.Content); i += 2 {
			sortEverything(node.Content[i])
		}
	case yaml.SequenceNode:
		for i := 0; i < len(node.Content); i++ {
			sortEverything(node.Content[i])
		}
	}
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

		sortEverything(&node)

		rankedKeySort(&find(&node, "metadata").Content, map[string]int{
			"name":              1,
			"annotations":       2,
			"labels":            3,
			"creationTimestamp": 4,
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
			"name":        1,
			"description": 2,
			"type":        3,
			"default":     4,
			"properties":  5,
			"enum":        6,
		})
		sortByName(spec, "results", map[string]int{
			"name":        1,
			"description": 2,
			"value":       3,
			"type":        4,
			"properties":  5,
		})
		sortByName(spec, "volumes", map[string]int{
			"name": 1,
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
			"env":                      7,
			"envFrom":                  8,
			"computeResources":         9,
			"volumeMounts":             10,
			"volumeDevices":            11,
			"workspaces":               12,
			"livenessProbe":            13,
			"readinessProbe":           14,
			"startupProbe":             15,
			"lifecycle":                16,
			"terminationMessagePath":   17,
			"terminationMessagePolicy": 18,
			"imagePullPolicy":          19,
			"securityContext":          20,
			"stdin":                    21,
			"stdinOnce":                22,
			"omitempty":                23,
			"script":                   24,
		})
		sortByName(spec, "stepTemplate", map[string]int{
			"image":            1,
			"command":          2,
			"args":             3,
			"workingDir":       4,
			"env":              5,
			"envFrom":          6,
			"computeResources": 7,
			"volumeMounts":     8,
			"volumeDevices":    9,
			"imagePullPolicy":  10,
			"securityContext":  11,
		})
		sortByName(find(spec, "stepTemplate"), "env", map[string]int{
			"name":      1,
			"value":     2,
			"valueFrom": 3,
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

			deleteIf(step, "computeResources", func(n *yaml.Node) bool {
				return n == nil || len(n.Content) == 0
			})
		}

		deleteIf(find(&node, "metadata"), "creationTimestamp", func(n *yaml.Node) bool {
			return n.Value == "null"
		})

		deleteIf(find(spec, "stepTemplate"), "computeResources", func(n *yaml.Node) bool {
			return n == nil || len(n.Content) == 0
		})

		unquote(&node)

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
