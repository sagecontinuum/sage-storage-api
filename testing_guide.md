# Testing Guide
This testing guide describes how to execute tests and how to find out
which percentage of codebase is covered by tests.

# Test Coverage
Find out which percentage of codebase is covered by tests and generate
file containing a graphical representation of test converage.

1. Make sure that line `entrypoint: sleep infinity` in `docker-compose.yaml` 
file is uncommented.

2. Clean docker volumes.

    `docker-compose down -v`

3. Compose and build the Docker container.

    `docker-compose up --build`

4. List all containers and get container ID of the current container.

    `docker ps`

5. Open shell inside the running container.

    `docker exec -ti <CONTAINER ID> /bin/bash`

6. Get percentage of statements that is covered by tests.

    `go test -race -covermode=atomic  -coverprofile=coverage.out`

7. Generate a file containing a graphical summary of covered statements and get path to the file.

    `go tool cover -html=coverage.out`

8. Exit the container's shell.

    `exit`

8. Copy the `coverage.out` file from the container to your computer.

    `docker cp <CONTAINER ID>:<PATH TO FILE> <PATH TO LOCATION WHERE FILE WILL BE SAVED>`
