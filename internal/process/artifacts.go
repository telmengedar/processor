package process

import (
	"embed"
	"fmt"
)

//go:embed artifacts/*
var examples embed.FS

func exampleText(path string) string {
	data, err := examples.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func GangolfV2(personaType string) (Blueprint, error) {
	if personaType == "" {
		return Blueprint{}, fmt.Errorf("explicit persona node type required")
	}
	var blueprint Blueprint
	if err := decode(exampleText("artifacts/gangolf-v2.json"), &blueprint); err != nil {
		return Blueprint{}, err
	}
	declaration := blueprint.Inputs["persona"]
	declaration.NodeType = personaType
	blueprint.Inputs["persona"] = declaration
	for i := range blueprint.Nodes {
		node := &blueprint.Nodes[i]
		if node.Type == "process-prompt-template" {
			path, ok := node.Body.(string)
			if !ok {
				return Blueprint{}, fmt.Errorf("invalid template artifact")
			}
			body, err := examples.ReadFile("artifacts/" + path)
			if err != nil {
				return Blueprint{}, err
			}
			node.Body = string(body)
		}
	}
	return blueprint, nil
}
