package api

import (
	"encoding/json"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
)

// Masking an answer the server built itself changes nothing, so an agent
// that already masks gets its answer through as it sent it.
func TestRemaskInspectKeepsMaskedAnswer(t *testing.T) {
	info := validContainerInspectResponse()
	info.Config.Env = []string{"DATABASE_URL=postgres://app:EnvPass1@db/app", "DB_PASSWORD=EnvPass2", "LOG_LEVEL=debug"}
	info.Config.Labels = map[string]string{"app.db.password": "LabelPass1", "tier": "web"}
	info.Config.Entrypoint = []string{"app", "--password"}
	info.Config.Cmd = []string{"CmdPass1", "--port", "80"}
	info.State.Health = &containertypes.Health{
		Status: "healthy",
		Log:    []*containertypes.HealthcheckResult{{Start: time.Unix(0, 0).UTC(), End: time.Unix(1, 0).UTC(), Output: "token=HealthPass1"}},
	}
	served, err := json.Marshal(buildInspectResponse(info))
	if err != nil {
		t.Fatal(err)
	}

	remasked, err := remaskInspect(served)
	if err != nil {
		t.Fatal(err)
	}

	var want, got any
	if err := json.Unmarshal(served, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(remasked, &got); err != nil {
		t.Fatal(err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("masking a masked answer changed it:\n got %s\nwant %s", gotJSON, wantJSON)
	}
}

// Fields an inspect answer leaves out stay out, and null ones stay null.
func TestRemaskInspectKeepsAbsentFields(t *testing.T) {
	got, err := remaskInspect([]byte(`{"name":"web","cmd":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"cmd":null,"name":"web"}` {
		t.Errorf("got %s", got)
	}
}

func TestRemaskInspectSkipsNullEnvEntries(t *testing.T) {
	got, err := remaskInspect([]byte(`{"env":[null,{"key":"DB_PASSWORD","value":"EnvPass1"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"env":[null,{"key":"DB_PASSWORD","value":"********"}]}` {
		t.Errorf("got %s", got)
	}
}

// A null health or health log stays null, and probe output that is not a
// string is dropped.
func TestRemaskInspectHealthEdges(t *testing.T) {
	tests := []struct{ body, want string }{
		{`{"health":null}`, `{"health":null}`},
		{`{"health":{"log":null}}`, `{"health":{"log":null}}`},
		{`{"health":{"log":[{"output":5,"start":"x"}],"status":"healthy"}}`, `{"health":{"log":[{"start":"x"}],"status":"healthy"}}`},
	}
	for _, tt := range tests {
		got, err := remaskInspect([]byte(tt.body))
		if err != nil {
			t.Fatalf("%s: %v", tt.body, err)
		}
		if string(got) != tt.want {
			t.Errorf("%s: got %s, want %s", tt.body, got, tt.want)
		}
	}
}

func TestRemaskInspectRefusesNonObjects(t *testing.T) {
	for _, body := range []string{`null`, `[]`, `"x"`, `{"env":"not a list"}`, `{"labels":[1]}`, `{"health":{"log":"x"}}`, `{"health":"x"}`} {
		if _, err := remaskInspect([]byte(body)); err == nil {
			t.Errorf("%s: accepted", body)
		}
	}
}
