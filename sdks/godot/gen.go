package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"
)

//go:generate go run gen.go
//go:generate gdformat addons/com.google.agones/AgonesAlpha.gd
//go:generate gdformat addons/com.google.agones/AgonesBeta.gd
//go:generate gdformat addons/com.google.agones/AgonesSdk.gd

const directory = "addons/com.google.agones"

var swagger_files = []string{"sdk", "alpha", "beta"}

func main() {

	if _, err := os.Stat(directory); os.IsNotExist(err) {
		os.MkdirAll(directory, os.ModePerm)
	}

	for _, swagger_file := range swagger_files {

		log.Println(strings.ToUpper(swagger_file))

		// Load swagger file
		var swagger map[string]map[string]interface{}
		in, _ := os.Open(fmt.Sprintf("../swagger/%s.swagger.json", swagger_file))
		b, _ := ioutil.ReadAll(in)
		json.Unmarshal(b, &swagger)

		// Create request template parser
		request_template, err := template.New("request.tpl").Funcs(template.FuncMap{
			// Pass helper functions
			"ToUpper": strings.ToUpper,
			"Replace": strings.Replace,
			// Function used to convert swagger types to Godot types
			"ToGodotType": func(v string) string {
				switch v {
				case "string":
					return "String"
				case "integer":
					return "int"
				case "number":
					return "float"
				case "boolean":
					return "bool"
				}
				return "Object"
			},
			// Get $ref value
			"GetSchemRef": func(schema map[string]interface{}) string {
				if val, ok := schema["$ref"]; ok {
					return val.(string)
				}
				return ""
			},
			// Build URL string object with formatting for Godot
			"ParseUrlParams": func(url string, params []interface{}) string {
				url = fmt.Sprintf("\"%s\"", url)

				pathVars := 0
				for _, key := range params {
					param, _ := key.(map[string]interface{})
					if param["in"] == "path" {
						// Clean url
						url = strings.Replace(url, fmt.Sprintf("{%s}", param["name"]), "%s", -1)

						// Add string formatting
						if pathVars == 0 {
							url += "% [" + param["name"].(string)
						} else {
							url += ", " + param["name"].(string)
						}
						pathVars += 1
					}
				}

				// Close params array
				if pathVars > 0 {
					url += "]"
				}
				return fmt.Sprintf("(%s)", url)
			},
			// Function used to get an array's max index
			"MaxIndex": func(list []interface{}) int {
				return len(list) - 1
			},
		}).ParseFiles("request.tpl")
		if err != nil {
			log.Fatalf("request.tpl parsing failed: %s", err)
		}

		// Loop through swagger paths
		var requests string = ""
		for key, value := range swagger["paths"] {
			var buffer bytes.Buffer
			err = request_template.Execute(&buffer, map[string]interface{}{
				"path": key,
				"data": value,
			})
			if err != nil {
				log.Fatalf("Parsing `%s` failed: %s", key, err)
			}
			requests += buffer.String()
			log.Printf("Parsing %s", key)
		}

		// Load client template
		client_template, err := template.New("template.tpl").ParseFiles("template.tpl")
		if err != nil {
			log.Fatalf("template.tpl parsing failed: %s", err)
		}

		// Open GD file
		out, err := os.Create(fmt.Sprintf("%s/Agones%s.gd", directory, strings.Title(swagger_file)))
		if err != nil {
			log.Fatalf("Failed to create Agones.gd: %s", err)
		}
		defer out.Close()

		// Write out final template
		err = client_template.Execute(out, map[string]interface{}{
			"header": "# This code is generated by go generate.\n# DO NOT EDIT BY HAND!",
			"data":   requests,
		})
		if err != nil {
			log.Fatalf("Execution failed: %s", err)
		}
	}
	log.Println("Complete!")
}
