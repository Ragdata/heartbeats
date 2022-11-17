package internal

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// StResponseStatus represents the response
type ResponseStatus struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// HeartbeatStatus represents a heartbeat status
type HeartbeatStatus struct {
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	LastPing time.Time `json:"lastPing"`
}

func (h *HeartbeatStatus) TimeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return CalculateAgo.Format(t)
}

// HandlerHome is the handler for the / endpoint
func HandlerHome(w http.ResponseWriter, req *http.Request) {
	outputFormat := req.URL.Query().Get("output")
	if outputFormat == "" {
		outputFormat = "txt"
	}
	msg := struct{ Message string }{Message: fmt.Sprintf("Welcome to the Heartbeat Server.\nVersion: %s", HeartbeatsServer.Version)}
	WriteOutput(w, http.StatusOK, outputFormat, msg, `Message: {{ .Message }}`)
}

// HandlerPing is the handler for the /ping/<heartbeat> endpoint
func HandlerPing(w http.ResponseWriter, req *http.Request) {
	outputFormat := req.URL.Query().Get("output")
	if outputFormat == "" {
		outputFormat = "txt"
	}

	vars := mux.Vars(req)
	heartbeatName := vars["heartbeat"]

	textTemplate := `{{ .Status }}`

	heartbeat, err := GetHeartbeatByName(heartbeatName)
	if err != nil {
		WriteOutput(w, http.StatusNotFound, outputFormat, err.Error(), textTemplate)
		return
	}

	heartbeat.GotPing()

	WriteOutput(w, http.StatusOK, outputFormat, &ResponseStatus{Status: "ok", Error: ""}, textTemplate)
}

// HandlerPingHelp is the handler for the /ping endpoint
func HandlerPingHelp(w http.ResponseWriter, req *http.Request) {
	outputFormat := req.URL.Query().Get("output")
	if outputFormat == "" {
		outputFormat = "txt"
	}

	n := rand.Int() % len(HeartbeatsServer.Heartbeats)

	usage := struct {
		Status string `json:"status"`
		Usage  string `json:"usage"`
	}{
		Status: "ok",
		Usage: fmt.Sprintf(`you must specify the name of the wanted heartbeat in the URL.
Example: %s/ping/%s`, HeartbeatsServer.Server.SiteRoot, HeartbeatsServer.Heartbeats[n].Name),
	}

	WriteOutput(w, http.StatusOK, outputFormat, &usage, `{{ .Usage }}`)
}

// HandlerState is the handler for the /status endpoint
func HandlerStatus(w http.ResponseWriter, req *http.Request) {
	outputFormat := req.URL.Query().Get("output")
	if outputFormat == "" {
		outputFormat = "txt"
	}

	vars := mux.Vars(req)
	heartbeatName := vars["heartbeat"]

	var txtFormat string
	txtFormat = `Name: {{ .Name }}
Status: {{ if .Status }}{{ .Status }}{{ else }}-{{ end }}
LastPing: {{ .TimeAgo .LastPing }}`

	if heartbeatName == "" {
		var h []HeartbeatStatus
		for _, heartbeat := range HeartbeatsServer.Heartbeats {
			s := &HeartbeatStatus{
				Name:     heartbeat.Name,
				Status:   heartbeat.Status,
				LastPing: heartbeat.LastPing,
			}
			h = append(h, *s)
		}

		textTmpl := fmt.Sprintf("%s%s\n%s", "{{ range . }}", txtFormat, "{{end}}")
		WriteOutput(w, http.StatusOK, outputFormat, &h, textTmpl)
		return
	}

	heartbeat, err := GetHeartbeatByName(heartbeatName)
	if err != nil {
		WriteOutput(w, http.StatusNotFound, outputFormat, ResponseStatus{Status: "nok", Error: err.Error()}, `Status: {{ .Status }} Error: {{  .Error }}`)
		return
	}

	heartbeatStates := &HeartbeatStatus{
		Name:     heartbeat.Name,
		Status:   heartbeat.Status,
		LastPing: heartbeat.LastPing,
	}

	WriteOutput(w, http.StatusOK, outputFormat, &heartbeatStates, txtFormat)
}

// HeartbeatsServer is the handler for the /healthz endpoint
func HandlerHealthz(w http.ResponseWriter, req *http.Request) {
	outputFormat := req.URL.Query().Get("output")
	if outputFormat == "" {
		outputFormat = "txt"
	}
	WriteOutput(w, http.StatusOK, outputFormat, &ResponseStatus{Status: "ok", Error: ""}, `{{ .Status }}`)
}

// WriteOutput writes the output to the response writer
func WriteOutput(w http.ResponseWriter, StatusCode int, outputFormat string, output interface{}, textTemplate string) {
	o, err := FormatOutput(outputFormat, textTemplate, output)
	if err != nil {
		w.WriteHeader(StatusCode)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(StatusCode)
	w.Write([]byte(o))
	return
}
