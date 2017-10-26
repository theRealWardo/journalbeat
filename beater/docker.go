// Copyright 2017 Matt Ward
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package beater

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/elastic/beats/libbeat/common"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/therealwardo/journalbeat/config"
)

func (jb *Journalbeat) initDocker() error {
	if !jb.config.DockerMetadata.Enabled {
		return nil
	}

	c := jb.config.DockerMetadata.Connection
	endpoint := c.Endpoint
	if len(endpoint) == 0 {
		endpoint = "unix:///var/run/docker.sock"
	}

	var d *docker.Client
	var err error
	if len(c.Cert) > 0 {
		d, err = docker.NewTLSClient(endpoint, c.Cert, c.Key, c.CA)
	} else {
		d, err = docker.NewClient(endpoint)
	}
	if err != nil {
		return err
	}
	jb.docker = d
	return nil
}

func (jb *Journalbeat) addDockerMetadata(event common.MapStr) {
	containerField := jb.config.DockerMetadata.ContainerIdField
	if len(containerField) == 0 {
		containerField = "CONTAINER_ID"
	}
	if cid, ok := event[containerField].(string); ok {
		container, ok := jb.containers[cid]
		if !ok {
			var err error
			container, err = jb.docker.InspectContainer(cid)
			if err != nil {
				// If there are any problems, we simply do not attach metdata.
				return
			}
			jb.containers[cid] = container
		}
		metadata := make(map[string]string)
		addDockerEnvMetadata(metadata, container, jb.config.DockerMetadata.Env)
		addDockerLabelsMetadata(metadata, container, jb.config.DockerMetadata.Labels)
		addDockerMetadata(metadata, container, jb.config.DockerMetadata.Metadata)
		if len(metadata) > 0 {
			event["docker"] = metadata
		}
	}
}

func addDockerEnvMetadata(m map[string]string, container *docker.Container, envs []string) {
	for _, v := range container.Config.Env {
		parts := strings.Split(v, "=")
		for _, env := range envs {
			if parts[0] == env {
				m[env] = parts[1]
			}
		}
	}
}

func addDockerLabelsMetadata(m map[string]string, container *docker.Container, labels []string) {
	for _, label := range labels {
		if v, ok := container.Config.Labels[label]; ok {
			m[label] = v
		}
	}
}

func addDockerMetadata(m map[string]string, container *docker.Container, fcfgs []config.DockerFormattedMetadataConfig) {
	for _, fcfg := range fcfgs {
		if fcfg.Template == nil {
			if t, err := template.New(fcfg.Field).Parse(fcfg.Format); err == nil {
				fcfg.Template = t
			}
		}
		if fcfg.Template != nil {
			b := new(bytes.Buffer)
			fcfg.Template.Execute(b, container)
			m[fcfg.Field] = b.String()
		}
	}
}
