package main

import (
	"encoding/json"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// ActionFunc describes the main action to run when this command is invoked
type PlaceHolderFunc func() string

type ServerStatus struct {
	Online                bool      `json:"online"`
	ViewerCount           int       `json:"viewerCount"`
	OverallMaxViewerCount int       `json:"overallMaxViewerCount"`
	SessionMaxViewerCount int       `json:"sessionMaxViewerCount"`
	LastConnectTime       time.Time `json:"lastConnectTime"`
	LastDisconnectTime    time.Time `json:"lastDisconnectTime"`
	VersionNumber         string    `json:"versionNumber"`
}

var (
	serverStatus *ServerStatus // singleton
)

var (
	Uptime PlaceHolderFunc = func() string {
		ss, err := getServerStatus()
		if err != nil {
			log.Errorln(err)
			return ""
		}

		diff := time.Now().Sub(ss.LastConnectTime)
		return diff.Truncate(time.Second).String()
	}
)

// getServerStatus discover when a server is broadcasting,
// the number of active viewers as well as
// other useful information for updating the user interface.
func getServerStatus() (*ServerStatus, error) {
	if serverStatus != nil {
		return serverStatus, nil
	}

	resp, err := http.DefaultClient.Get(ServerURL + "/api/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ss ServerStatus
	if err := json.NewDecoder(resp.Body).Decode(&ss); err != nil {
		return nil, err
	}

	serverStatus = &ss
	return &ss, nil
}
