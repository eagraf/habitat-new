package docker

import "github.com/docker/docker/client"

func NewDockerDriver() (*DockerDriver, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &DockerDriver{
		client: dockerClient,
	}, nil
}
