package main

import (
  "strconv"
  "net/http"
  "log"
  "bytes"
  "encoding/json"
  "io/ioutil"
)

type Project struct {
	ProjectID          int       `json:"project_id"`
	Name               string    `json:"name"`
}

type JsonInput struct {
	Targets    []Targets `json:"targets"`
	EventTypes []string `json:"event_types"`
  Enabled    bool `json:"enabled"`
}

type Targets struct {
	Type           string `json:"type"`
  Address        string `json:"address"`
  AuthHeader     string `json:"auth_header"`
  SkipCertVerify bool   `json:"skip_cert_verify"`
}

func main() {

  client := &http.Client{
  }

  address := "http://5dc2f76a.ngrok.io/webhook"
  name := "webhook-test"
  projectId := ""

  bodyTargets := []Targets {
    Targets{
      Type: "http",
      Address: address,
      AuthHeader: "auth_header",
      SkipCertVerify: true,
    },
  }

  body := &JsonInput{
    Targets: bodyTargets,
    EventTypes: []string {
      "pushImage",
      "scanningFailed",
      "scanningCompleted",
    },
    Enabled: true,
  }


  bodyJson, err := json.Marshal(body)

  //log.Print(bytes.NewBuffer(bodyJson))

  request, err := http.NewRequest("GET", "https://harbor.toolchain.lead.sandbox.liatr.io/api/projects/", nil)
  apiResponse, err := client.Do(request)
  if err != nil {
    log.Print(err)
  }

  projectList, err := ioutil.ReadAll(apiResponse.Body)

  var project []Project

  json.Unmarshal([]byte(projectList), &project)

  for _, p := range project {
    if p.Name == name {
      projectId = strconv.Itoa(p.ProjectID)
    }
  }

  if projectId != ""{
    url := "https://harbor.toolchain.lead.sandbox.liatr.io/api/projects/" + projectId + "/webhook/policies"
    
    log.Print(url)

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyJson))
    req.SetBasicAuth("", "")
    //req.Header.Add("If-None-Match", `W/"wyzzy"`)
    resp, err := client.Do(req)
    if err != nil {
      log.Print(err)
    }
    log.Print(resp)
    // ...
    
    defer resp.Body.Close()
  }


}
