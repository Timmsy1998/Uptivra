package server_test

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"uptivra/internal/domain"
	"uptivra/internal/server"
	"uptivra/internal/store"
)

// Keep the published wire contract aligned with routes and serialized response fields.
func TestOpenAPIContract(t *testing.T) {
	data, err := os.ReadFile("../../docs/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		OpenAPI    string                                `json:"openapi"`
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas map[string]struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err = json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.OpenAPI != "3.1.0" {
		t.Fatal("expected OpenAPI 3.1")
	}
	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	routes := regexp.MustCompile(`mux\.Handle(?:Func)?\("(GET|POST|PUT|DELETE) ([^"]+)"`).FindAllStringSubmatch(string(source), -1)
	count := 0
	for _, operations := range spec.Paths {
		count += len(operations)
	}
	if count != len(routes) {
		t.Fatalf("documented %d operations, registered %d", count, len(routes))
	}
	for _, route := range routes {
		if spec.Paths[route[2]][strings.ToLower(route[1])] == nil {
			t.Errorf("undocumented route %s %s", route[1], route[2])
		}
	}
	responses := map[string]any{"Maintenance": domain.Maintenance{}, "Monitor": domain.Monitor{}, "Check": domain.Check{}, "Incident": domain.Incident{}, "IncidentUpdate": domain.IncidentUpdate{}, "Settings": domain.Settings{}, "PublicMonitor": server.PublicMonitor{}, "PublicStatus": server.PublicStatus{}, "Key": store.Key{}, "DeliveryStatus": store.DeliveryStatus{}, "SchedulerRun": store.SchedulerRun{}, "SchedulerHealth": store.SchedulerHealth{}}
	for name, value := range responses {
		properties := spec.Components.Schemas[name].Properties
		typ := reflect.TypeOf(value)
		fields := 0
		for i := 0; i < typ.NumField(); i++ {
			field := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
			if field == "-" {
				continue
			}
			fields++
			if properties[field] == nil {
				t.Errorf("%s.%s missing from contract", name, field)
			}
		}
		if fields != len(properties) {
			t.Errorf("%s schema contains undocumented wire differences: %d fields, %d properties", name, fields, len(properties))
		}
	}
	for _, secret := range []string{"url", "headers", "authorized", "contains"} {
		if spec.Components.Schemas["PublicMonitor"].Properties[secret] != nil {
			t.Errorf("public contract exposes %s", secret)
		}
	}
}
