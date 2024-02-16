# Habiat

## External Tools
To run all the code and tooling in this repository, you will need to have the following installe:
* GNU Make > v4 (this should be available through xcode)
* Docker Engine and the Docker CLI: https://docs.docker.com/engine/install/
* golangci-lint: https://golangci-lint.run/usage/install/
* Postman: https://www.postman.com/downloads/

## Local Development
The local dev setup runs the main Habitat node in a docker container. The code is built inside the container using [Air](https://github.com/cosmtrek/air), which allows for live reloading. To build the dev container, run:
```
make docker-build
```
Rebuilds are required when new dependencies are added to this repository's go.mod file, or if the Dockerfile is changed.

Before running the node, we need to do some extra setup to get our dev certificates volumed into the container. These 
certificates are used to authenticate you and the node when in dev mode. To do this, run:
```
make install
```

Now you are able to start Habitat. To run the node, run one of the following:
```
make run-dev          # Runs the habitat node in dev mode
```
The container saves node state in an anonymous volume. If you'd like to run the Habitat node with completely new state, run:
```
make run-dev-fresh
```
You should now see a bunch of logs indicating the node has come up.

### Setting up Postman
Now that your node is running, let's set up Postman so we can make requests to the Habitat API. Habitat uses an mTLS for authentication, which means that clients must supply a certificate that the server is able to authenticate. In the previous steps, we already supplied our dev certificate to the server, but now we need to add it to Postman so that it can
submit it along with the request.

Go to `Postman > Preferences > Certificates` and press `Add Certificate`. Fill in the following fields:
```
HOST: localhost:3000
CRT File: <full path to habitat>/.habitat/certificates/dev_node_cert.pem`
KEY File: <full path to habitat>/.habitat/certificates/dev_node_cert.key`
```
Now Postman will submit all requests over HTTPS with your certificate. Try `GET https://localhost:3000/node` to verify this is working. To use other API endpoints, look at their handlers to determine the required input and expected output.

## Testing
To run all unit tests, run:
```
make test
```
There is also a make-rule for getting test coverage, which will open a file in your browser showing the coverage information for various files:
```
make test-coverage
```
This repository uses [gomock](https://github.com/uber-go/mock) to create mocks in tests. Generally, mocks will be generated with a command looking like this that writes the mock code into a `mocks` package sitting next to where the real code lives:
```
mockgen -source=internal/node/hdb/dbms.go -package mocks > internal/node/hdb/mocks/mock_dbms.go
```
The mocks can be regenerated as needed when the interface they are mocking is changed. 


## Architecture
This repository follows the [standard go project structure](https://github.com/golang-standards/project-layout). The only major difference from the standard layout is that this repository contains a `core` folder which houses structs used across the application. This folder should not have any dependencies besides the standard library.


 The main application framework is the [Fx](https://uber-go.github.io/fx/) dependency injetion framework. This allows for easy wiring together of components, and testability when components are defined as interfaces. 

