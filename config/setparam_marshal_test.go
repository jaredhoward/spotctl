package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func marshalParams(t *testing.T, params map[string]SetParam) string {
	t.Helper()
	out, err := yaml.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestSetParamMarshal_ShortForms(t *testing.T) {
	tests := []struct {
		name  string
		param SetParam
		want  string
	}{
		{"pool only is a bare sequence",
			SetParam{Pool: []PoolEntry{{URI: "a"}, {URI: "b"}}},
			"p:\n    - uri: a\n    - uri: b\n"},
		{"pool entry overrides are kept",
			SetParam{Pool: []PoolEntry{{URI: "a", Volume: intPtr(25)}, {URI: "b", Shuffle: boolPtr(false)}}},
			"p:\n    - uri: a\n      volume: 25\n    - uri: b\n      shuffle: false\n"},
		{"whole number default is a bare number", SetParam{Default: "40"}, "p: 40\n"},
		{"true default is a bare bool", SetParam{Default: "true"}, "p: true\n"},
		{"false default is a bare bool", SetParam{Default: "false"}, "p: false\n"},
		{"string default is a scalar", SetParam{Default: "context"}, "p: context\n"},
		{"template default is a scalar", SetParam{Default: "{{ volume }}"}, "p: '{{ volume }}'\n"},
		{"required keeps the mapping", SetParam{Required: true}, "p:\n    required: true\n"},
		{"required with default keeps the mapping", SetParam{Required: true, Default: "40"}, "p:\n    default: \"40\"\n    required: true\n"},
		{"empty is an empty mapping", SetParam{}, "p: {}\n"},
		{"pool with a default keeps the mapping so nothing is lost",
			SetParam{Pool: []PoolEntry{{URI: "a"}}, Default: "a"},
			"p:\n    default: a\n    pool:\n        - uri: a\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := marshalParams(t, map[string]SetParam{"p": tc.param}); got != tc.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

func TestSetParamMarshal_RoundTrips(t *testing.T) {
	params := map[string]SetParam{
		"pool":     {Pool: []PoolEntry{{URI: "u1"}, {URI: "u2", Volume: intPtr(30), Repeat: strPtr("off")}}},
		"number":   {Default: "40"},
		"negative": {Default: "-5"},
		"bool":     {Default: "true"},
		"word":     {Default: "context"},
		"off":      {Default: "off"},
		"leading0": {Default: "007"},
		"decimal":  {Default: "1.5"},
		"template": {Default: "{{ level }}"},
		"required": {Required: true},
		"both":     {Required: true, Default: "x"},
		"empty":    {},
		"poolplus": {Pool: []PoolEntry{{URI: "u"}}, Default: "u"},
	}
	out, err := yaml.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]SetParam
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatalf("saved output does not load: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(params, back) {
		t.Errorf("round trip changed the params:\n got %#v\nwant %#v\nyaml:\n%s", back, params, out)
	}
}

// A config written in the documented flat form, or in the nested form older
// versions saved, must load identically, and saving must produce the flat form.
func TestSaveWritesFlatPoolAndScalars(t *testing.T) {
	flat := `client_id: id
client_secret: s
refresh_token: r
redirect_uri: http://localhost/cb
sets:
    demo:
        device_id: '{{ device }}'
        params:
            device: {}
            pool:
                - uri: spotify:playlist:AAAA
                - uri: spotify:playlist:BBBB
                  volume: 25
            method: date
            volume: 40
        commands:
            - action: play
              params:
                uri: '{{ uri }}'
`
	legacy := strings.Replace(flat, "            pool:\n                - uri: spotify:playlist:AAAA\n                - uri: spotify:playlist:BBBB\n                  volume: 25\n",
		"            pool:\n                pool:\n                    - uri: spotify:playlist:AAAA\n                    - uri: spotify:playlist:BBBB\n                      volume: 25\n", 1)
	legacy = strings.Replace(legacy, "            method: date\n            volume: 40\n",
		"            method:\n                default: date\n            volume:\n                default: \"40\"\n", 1)
	if legacy == flat {
		t.Fatal("test setup: legacy fixture was not built")
	}

	dir := t.TempDir()
	var loaded []*Config
	for name, src := range map[string]string{"flat": flat, "legacy": legacy} {
		in := filepath.Join(dir, name+"_in.yaml")
		if err := os.WriteFile(in, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		loaded = append(loaded, cfg)

		out := filepath.Join(dir, name+"_out.yaml")
		if err := Save(out, cfg); err != nil {
			t.Fatal(err)
		}
		saved, _ := os.ReadFile(out)
		text := string(saved)
		for _, want := range []string{"            method: date\n", "            volume: 40\n", "            pool:\n                - uri: spotify:playlist:AAAA\n"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: saved config missing %q:\n%s", name, want, text)
			}
		}
		if strings.Contains(text, "pool:\n                pool:") || strings.Contains(text, "default:") {
			t.Errorf("%s: saved config still uses the expanded form:\n%s", name, text)
		}
		again, err := Load(out)
		if err != nil {
			t.Fatalf("%s: saved config does not load: %v", name, err)
		}
		if !reflect.DeepEqual(cfg.Sets, again.Sets) {
			t.Errorf("%s: sets changed across save and load:\n got %#v\nwant %#v", name, again.Sets, cfg.Sets)
		}
	}
	if !reflect.DeepEqual(loaded[0].Sets, loaded[1].Sets) {
		t.Error("flat and legacy forms should load to the same config")
	}
}

func TestScalarDefault(t *testing.T) {
	tests := []struct {
		in   string
		want interface{}
	}{
		{"40", 40}, {"0", 0}, {"-5", -5}, {"true", true}, {"false", false},
		{"007", "007"}, {"+5", "+5"}, {"1.5", "1.5"}, {"off", "off"}, {"True", "True"}, {"", ""},
	}
	for _, tc := range tests {
		if got := scalarDefault(tc.in); got != tc.want {
			t.Errorf("scalarDefault(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }
